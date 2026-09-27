package httpapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
	"strconv"
	"time"
)

func (c *Controller) patchCapability(op string) (patchcontrol.Reply, error) {
	t, ok := c.source.(patchcontrol.Transport)
	if !ok || c.source.Name() != "serial" {
		return patchcontrol.Reply{}, patchcontrol.ErrLocked
	}
	if live, ok := c.source.(interface{ PatchLive() bool }); !ok || !live.PatchLive() {
		return patchcontrol.Reply{}, patchcontrol.ErrLocked
	}
	// Older measurement-only firmware is not sent commands unless it explicitly
	// advertises capability. The shipped unprovisioned device remains locked.
	e, err := c.source.Latest(context.Background())
	if err != nil || e.Patch == nil || !e.Patch.Capable {
		return patchcontrol.Reply{}, patchcontrol.ErrLocked
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	r, err := t.PatchExchange(ctx, patchcontrol.Command{Op: op, NowMS: time.Now().UnixMilli()})
	if err != nil {
		return r, err
	}
	if !r.Qualified() || r.DeviceID != e.DeviceID || r.ProfileID != e.ProfileID || r.BootID != strconv.FormatUint(uint64(e.Patch.BootID), 10) {
		return r, patchcontrol.ErrLocked
	}
	profile, err := c.repository.GetProfile(r.ProfileID)
	if err != nil || profile == nil || !profile.Confirmed || profile.Version != r.ProfileRevision || passport.MappingHash(*profile) != r.ProbeMapHash {
		return r, patchcontrol.ErrLocked
	}
	return r, nil
}
func (c *Controller) patchQualified(p patchcontrol.Parameters) bool {
	r, e := c.patchCapability("status")
	return e == nil && r.DeviceID == p.DeviceID && r.BootID == p.BootID && r.ProfileID == p.ProfileID && r.ProfileRevision == p.ProfileRevision && r.ProbeMapHash == p.ProbeMapHash && r.Pin == p.PatchPin && r.TargetNode == p.TargetNode
}
func (c *Controller) patchMaster(p patchcontrol.Parameters) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.patchEnabled[p.ProfileID] == p.DeviceID+":"+p.BootID+":"+p.ProbeMapHash
}
func (c *Controller) physicalPatchDriver(p patchcontrol.Parameters) (*patchcontrol.WireDriver, error) {
	r, err := c.patchCapability("status")
	if err != nil {
		return nil, err
	}
	profile, err := c.repository.GetProfile(p.ProfileID)
	if err != nil {
		return nil, err
	}
	if profile == nil || !profile.Confirmed || profile.Version != p.ProfileRevision || passport.MappingHash(*profile) != p.ProbeMapHash {
		return nil, errors.New("profile revision/mapping changed")
	}
	allowed, _, _, err := c.physicalCalibrationContext(*profile)
	if err != nil || !allowed {
		return nil, errors.New("confirmed physical probe setup required")
	}
	targets := []string{}
	for _, connection := range profile.Connections {
		if connection.Confirmed && connection.Target == p.TargetNode && connection.Probe != "" {
			if _, ok := profile.Probe(connection.Probe); ok {
				targets = append(targets, connection.Probe)
			}
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("qualified target must have a measurement probe for VERIFY")
	}
	measurements, ok := c.repository.(domain.MeasurementRepository)
	if !ok {
		return nil, errors.New("measurement storage unavailable")
	}
	d := &patchcontrol.WireDriver{Transport: c.source.(patchcontrol.Transport), Capability: r}
	d.Current = func() patchcontrol.DeviceContext {
		e, err := c.source.Latest(context.Background())
		alive := err == nil && e.Patch != nil && e.Patch.Capable && e.DeviceID == p.DeviceID && e.ProfileID == p.ProfileID && strconv.FormatUint(uint64(e.Patch.BootID), 10) == p.BootID
		if live, ok := c.source.(interface{ PatchLive() bool }); ok {
			alive = alive && live.PatchLive()
		} else {
			alive = false
		}
		current, err := c.repository.GetProfile(p.ProfileID)
		valid := err == nil && current != nil && current.Confirmed && current.Version == p.ProfileRevision && passport.MappingHash(*current) == p.ProbeMapHash
		return patchcontrol.DeviceContext{Ready: r.Qualified() && valid, Connected: alive, MasterEnabled: c.patchMaster(p), Source: "REAL_SERIAL", DeviceID: p.DeviceID, BootID: p.BootID, ProfileID: p.ProfileID, ProbeMapHash: p.ProbeMapHash, ProfileRevision: p.ProfileRevision, DedicatedPin: r.Pin, AllowedNodes: map[string]bool{r.TargetNode: true}}
	}
	var before, after domain.MeasurementWindow
	evidence := func(w domain.MeasurementWindow, start int64) patchcontrol.Evidence {
		return patchcontrol.Evidence{MeasurementID: w.ID, Source: "REAL_SERIAL", DeviceID: w.DeviceID, BootID: p.BootID, ProfileID: w.ProfileID, ProfileRevision: profile.Version, ProbeMapHash: passport.MappingHash(*profile), Sequence: w.Sequence, WindowStartedAtMS: start}
	}
	d.CaptureBefore = func(ctx context.Context) (patchcontrol.Evidence, error) {
		e, err := c.source.Latest(ctx)
		if err != nil {
			return patchcontrol.Evidence{}, err
		}
		enriched, err := c.profileWithKnownGood(*profile, e)
		if err != nil {
			return patchcontrol.Evidence{}, err
		}
		*profile = enriched
		before, err = c.captureTestWindow(ctx, measurements, *profile, nil)
		return evidence(before, time.Now().UnixMilli()-int64(before.Raw.WindowMS)), err
	}
	d.CaptureFresh = func(ctx context.Context, receipt patchcontrol.Receipt) (patchcontrol.Evidence, error) {
		fresh, ok := c.source.(domain.FreshTelemetrySource)
		if !ok {
			return patchcontrol.Evidence{}, errors.New("fresh real capture unavailable")
		}
		seq := before.Sequence
		for {
			state := d.Context()
			if !state.Ready || !state.Connected {
				return patchcontrol.Evidence{}, errors.New("hardware/profile changed before post-action capture")
			}
			e, err := fresh.WaitNext(ctx, seq)
			if err != nil {
				return patchcontrol.Evidence{}, err
			}
			seq = e.Sequence
			if e.DeviceID != p.DeviceID || e.ProfileID != p.ProfileID || e.Patch == nil || strconv.FormatUint(uint64(e.Patch.BootID), 10) != p.BootID {
				return patchcontrol.Evidence{}, errors.New("post-action device/boot/profile changed")
			}
			if uint64(e.UptimeMS) <= uint64(e.WindowMS)+d.DisabledUptimeMS {
				continue
			}
			analysis, err := signalanalysis.New().Analyze(e, *profile)
			if err != nil {
				return patchcontrol.Evidence{}, err
			}
			after, err = measurements.SaveMeasurement(domain.MeasurementWindow{ProfileID: profile.ID, DeviceID: e.DeviceID, Source: "serial", Sequence: e.Sequence, Raw: e, Analysis: analysis, CapturedAtMS: e.CapturedAtMS})
			return evidence(after, receipt.DisabledAtMS+int64(uint64(e.UptimeMS)-uint64(e.WindowMS)-d.DisabledUptimeMS)), err
		}
	}
	d.Compare = func(ctx context.Context, b, a patchcontrol.Evidence) (string, error) {
		if !d.Context().Ready {
			return "", errors.New("profile changed before verification")
		}
		if b.MeasurementID != before.ID || a.MeasurementID != after.ID {
			return "", errors.New("verification capture identity changed")
		}
		result := testplanner.New().Verify(*profile, targets, before, after)
		return fmt.Sprintf("%s: %s (Before #%d / After #%d, REAL_SERIAL)", result.Status, result.Summary, before.ID, after.ID), nil
	}
	return d, nil
}
