package patchcontrol

import (
	"context"
	"errors"
	"time"
)

// The serial protocol is separate from measurement Telemetry v2. No raw GPIO
// endpoint exists: ARM binds the entire approved action to a boot challenge.
type Command struct {
	Type      string  `json:"type"`
	RequestID string  `json:"request_id"`
	Op        string  `json:"op"`
	NowMS     int64   `json:"now_ms"`
	Challenge string  `json:"challenge"`
	Action    *Action `json:"action,omitempty"`
	ActionID  string  `json:"action_id,omitempty"`
	Digest    string  `json:"digest,omitempty"`
}
type Reply struct {
	Type            string `json:"type"`
	RequestID       string `json:"request_id"`
	OK              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
	State           string `json:"state"`
	Challenge       string `json:"challenge"`
	DeviceID        string `json:"device_id"`
	BootID          string `json:"boot_id"`
	ProfileID       string `json:"profile_id"`
	ProfileRevision int    `json:"profile_revision"`
	ProbeMapHash    string `json:"probe_map_hash"`
	QualificationID string `json:"qualification_id"`
	Pin             int    `json:"pin"`
	TargetNode      string `json:"target_node"`
	ActionID        string `json:"action_id"`
	Digest          string `json:"digest"`
	UptimeMS        uint64 `json:"uptime_ms"`
	Completed       bool   `json:"completed"`
}
type Transport interface {
	PatchExchange(context.Context, Command) (Reply, error)
}

// A READY reply must originate in the provisioned firmware: qualification ID,
// physical interlock, dedicated pins and boot challenge are checked there.
func (r Reply) Qualified() bool {
	p := Parameters{ProfileID: r.ProfileID, ProfileRevision: r.ProfileRevision, ProbeMapHash: r.ProbeMapHash, DeviceID: r.DeviceID, BootID: r.BootID, TargetNode: r.TargetNode, PatchPin: r.Pin, Mode: "PULSE", LogicLevel: "HIGH", MaxVoltage: 3.3, DurationMS: 10, ExpiresAtMS: time.Now().UnixMilli() + 1000, Source: "REAL_SERIAL"}
	stateOK := r.State == "READY" || r.State == "DISABLED" || r.State == "ARMED" || r.State == "ACTIVE"
	return r.OK && r.QualificationID != "" && len(r.Challenge) == 32 && stateOK && Validate(p, time.Now().UnixMilli()) == nil
}

// WireDriver delegates capture/VERIFY to the existing backend, never firmware.
type WireDriver struct {
	Transport        Transport
	Current          func() DeviceContext
	CaptureBefore    func(context.Context) (Evidence, error)
	CaptureFresh     func(context.Context, Receipt) (Evidence, error)
	Compare          func(context.Context, Evidence, Evidence) (string, error)
	Capability       Reply
	DisabledUptimeMS uint64
}

func (d *WireDriver) Context() DeviceContext                     { return d.Current() }
func (d *WireDriver) Before(c context.Context) (Evidence, error) { return d.CaptureBefore(c) }
func (d *WireDriver) CaptureAfter(c context.Context, r Receipt) (Evidence, error) {
	return d.CaptureFresh(c, r)
}
func (d *WireDriver) Verify(c context.Context, b, a Evidence) (string, error) {
	return d.Compare(c, b, a)
}
func (d *WireDriver) exchange(c context.Context, op string, a *Action, id, digest string) (Reply, error) {
	r, e := d.Transport.PatchExchange(c, Command{Type: "patch_command", Op: op, Challenge: d.Capability.Challenge, Action: a, ActionID: id, Digest: digest})
	if e == nil && (!r.OK || r.Challenge != d.Capability.Challenge || r.BootID != d.Capability.BootID || r.DeviceID != d.Capability.DeviceID || r.ProfileID != d.Capability.ProfileID || r.ProfileRevision != d.Capability.ProfileRevision || r.ProbeMapHash != d.Capability.ProbeMapHash || r.Pin != d.Capability.Pin || r.TargetNode != d.Capability.TargetNode || r.QualificationID != d.Capability.QualificationID) {
		e = errors.New("PATCH device rejected command or boot/session changed: " + r.Error)
	}
	return r, e
}
func (d *WireDriver) Disable(c context.Context, id string) error {
	r, e := d.exchange(c, "disable", nil, id, "")
	if e == nil && r.State != "DISABLED" && r.State != "LOCKED" {
		return errors.New("output disable not acknowledged")
	}
	return e
}
func (d *WireDriver) Execute(c context.Context, a Action) (Receipt, error) {
	if a.Parameters.Mode != "PULSE" && a.Parameters.Mode != "DIGITAL" {
		return Receipt{}, errors.New("provisioned driver supports bounded digital/pulse only")
	}
	r, e := d.exchange(c, "arm", &a, a.ID, a.Digest)
	if e != nil {
		return Receipt{}, e
	}
	if r.State != "ARMED" {
		return Receipt{}, errors.New("arming not acknowledged")
	}
	r, e = d.exchange(c, "execute", nil, a.ID, a.Digest)
	if e != nil {
		return Receipt{}, e
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if r.ActionID != a.ID || r.Digest != a.Digest {
			return Receipt{}, errors.New("device action binding mismatch")
		}
		if r.State == "DISABLED" {
			if !r.Completed || r.UptimeMS == 0 {
				return Receipt{}, errors.New("firmware disabled output without completing approved duration")
			}
			d.DisabledUptimeMS = r.UptimeMS
			return Receipt{a.ID, a.Parameters.BootID, true, time.Now().UnixMilli()}, nil
		}
		if r.State != "ACTIVE" {
			return Receipt{}, errors.New("unexpected device state: " + r.State)
		}
		select {
		case <-c.Done():
			return Receipt{}, c.Err()
		case <-ticker.C:
		}
		r, e = d.exchange(c, "poll", nil, a.ID, a.Digest)
		if e != nil {
			return Receipt{}, e
		}
	}
}
