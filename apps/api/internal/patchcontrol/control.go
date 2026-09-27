// Package patchcontrol owns active-test authorization. It is deliberately
// separate from PROBE and measurement ingestion. Production has no physical
// driver: neither a connected serial device nor an environment flag unlocks it.
package patchcontrol

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const MaxDurationMS = 250
const MaxApprovalAgeMS = 15000
const PhysicalInterfaceVerified = false

var ErrLocked = errors.New("PATCH_LOCKED: dedicated output/protection stage and physical driver are not verified")

type Parameters struct {
	ProfileID       string  `json:"profile_id"`
	ProfileRevision int     `json:"profile_revision"`
	ProbeMapHash    string  `json:"probe_map_hash"`
	DeviceID        string  `json:"device_id"`
	BootID          string  `json:"boot_id"`
	TargetNode      string  `json:"target_node"`
	PatchPin        int     `json:"patch_pin"`
	Mode            string  `json:"mode"`
	LogicLevel      string  `json:"logic_level"`
	MaxVoltage      float64 `json:"max_voltage"`
	FrequencyHz     float64 `json:"frequency_hz"`
	DutyCycle       float64 `json:"duty_cycle"`
	DurationMS      int     `json:"duration_ms"`
	ExpiresAtMS     int64   `json:"expires_at_ms"`
	Source          string  `json:"source"`
}

type Event struct {
	State  string `json:"state"`
	AtMS   int64  `json:"at_ms"`
	Detail string `json:"detail"`
}
type Approval struct {
	Actor       string `json:"actor"`
	Digest      string `json:"digest"`
	AtMS        int64  `json:"at_ms"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}
type Evidence struct {
	MeasurementID     int64  `json:"measurement_id"`
	Source            string `json:"source"`
	DeviceID          string `json:"device_id"`
	BootID            string `json:"boot_id"`
	ProfileID         string `json:"profile_id"`
	ProfileRevision   int    `json:"profile_revision"`
	ProbeMapHash      string `json:"probe_map_hash"`
	Sequence          uint64 `json:"sequence"`
	WindowStartedAtMS int64  `json:"window_started_at_ms"`
}
type Action struct {
	ID         string     `json:"id"`
	Owner      string     `json:"owner"`
	Parameters Parameters `json:"parameters"`
	Digest     string     `json:"digest"`
	State      string     `json:"state"`
	Approval   *Approval  `json:"approval,omitempty"`
	Events     []Event    `json:"events"`
	Before     *Evidence  `json:"before,omitempty"`
	After      *Evidence  `json:"after,omitempty"`
	Result     string     `json:"result,omitempty"`
}

// SavePatchAction must persist atomically; IDs are server-generated. An action
// is immutable once proposed; altered parameters require a new action/approval.
type Store interface {
	SavePatchAction(Action) error
	GetPatchAction(string) (*Action, error)
	ListPatchActions(string) ([]Action, error)
	InterruptPatchActions(int64) error
}

// Context is supplied by a trusted adapter, never decoded from an HTTP body.
// Ready means an explicit capability + boot/session handshake was verified.
type DeviceContext struct {
	Ready, Connected, MasterEnabled                   bool
	Source, DeviceID, BootID, ProfileID, ProbeMapHash string
	ProfileRevision, DedicatedPin                     int
	AllowedNodes                                      map[string]bool
}
type Receipt struct {
	ActionID, BootID string
	Disabled         bool
	DisabledAtMS     int64
}
type Driver interface {
	Context() DeviceContext
	Before(context.Context) (Evidence, error)
	Execute(context.Context, Action) (Receipt, error)
	Disable(context.Context, string) error
	CaptureAfter(context.Context, Receipt) (Evidence, error)
	Verify(context.Context, Evidence, Evidence) (string, error)
}

type Controller struct {
	mu               sync.Mutex
	store            Store
	now              func() time.Time
	physicalVerified bool
}

func New(store Store) (*Controller, error) {
	if err := store.InterruptPatchActions(time.Now().UnixMilli()); err != nil {
		return nil, err
	}
	return &Controller{store: store, now: time.Now, physicalVerified: PhysicalInterfaceVerified}, nil
}

func Digest(p Parameters) string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func Validate(p Parameters, now int64) error {
	if p.ProfileID == "" || len(p.ProfileID) > 64 || p.ProfileRevision < 1 || p.ProbeMapHash == "" || len(p.ProbeMapHash) > 128 || p.DeviceID == "" || len(p.DeviceID) > 64 || p.BootID == "" || len(p.BootID) > 64 || p.TargetNode == "" || len(p.TargetNode) > 128 {
		return errors.New("missing or invalid device/profile/revision/map/target binding")
	}
	if p.Source != "REAL_SERIAL" && p.Source != "SIMULATED" {
		return errors.New("invalid source provenance")
	}
	if p.PatchPin < 0 || p.PatchPin > 48 {
		return errors.New("unsupported PATCH pin")
	}
	// Measurement pins, OLED bus, USB and flash pins can never be PATCH.
	switch p.PatchPin {
	case 8, 3, 16, 21, 9, 48, 17, 18, 19, 20, 0, 45, 46:
		return errors.New("reserved/measurement pin cannot be PATCH")
	}
	if p.PatchPin >= 26 && p.PatchPin <= 37 {
		return errors.New("flash pin cannot be PATCH")
	}
	if math.IsNaN(p.MaxVoltage) || math.IsInf(p.MaxVoltage, 0) || p.MaxVoltage != 3.3 {
		return errors.New("only a qualified 3.3 V interface is modeled; no 5 V output")
	}
	if p.LogicLevel != "LOW" && p.LogicLevel != "HIGH" {
		return errors.New("unsupported logic level")
	}
	if p.DurationMS < 1 || p.DurationMS > MaxDurationMS {
		return errors.New("duration must be 1-250 ms")
	}
	if p.ExpiresAtMS <= now || p.ExpiresAtMS > now+60000 {
		return errors.New("action expiry must be within 60 seconds")
	}
	if math.IsNaN(p.FrequencyHz) || math.IsInf(p.FrequencyHz, 0) || math.IsNaN(p.DutyCycle) || math.IsInf(p.DutyCycle, 0) {
		return errors.New("non-finite waveform")
	}
	switch p.Mode {
	case "DIGITAL", "PULSE":
		if p.FrequencyHz != 0 || p.DutyCycle != 0 {
			return errors.New("frequency/duty not allowed for this mode")
		}
	case "PULSE_TRAIN":
		if p.LogicLevel != "HIGH" || p.FrequencyHz < 1 || p.FrequencyHz > 100 || p.DutyCycle < .1 || p.DutyCycle > .9 || p.FrequencyHz*float64(p.DurationMS) < 1000 {
			return errors.New("pulse train requires 1-100 Hz, 10-90% duty, HIGH, and at least one cycle")
		}
	default:
		return errors.New("unsupported output mode")
	}
	return nil
}

func (c *Controller) event(a *Action, state, detail string) error {
	a.State = state
	a.Events = append(a.Events, Event{state, c.now().UnixMilli(), detail})
	return c.store.SavePatchAction(*a)
}

func (c *Controller) Propose(p Parameters, actor string) (*Action, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if actor == "" {
		return nil, errors.New("verified actor required")
	}
	if err := Validate(p, c.now().UnixMilli()); err != nil {
		return nil, err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	a := &Action{ID: hex.EncodeToString(id), Owner: actor, Parameters: p, Digest: Digest(p)}
	if err := c.event(a, "PROPOSED", "parameters recorded; no output"); err != nil {
		return nil, err
	}
	if err := c.event(a, "VALIDATED", "schema/limits validated; not physical safety certification"); err != nil {
		return nil, err
	}
	if p.Source == "REAL_SERIAL" && !c.physicalVerified {
		if err := c.event(a, "LOCKED", ErrLocked.Error()); err != nil {
			return nil, err
		}
		return a, ErrLocked
	}
	return a, c.event(a, "AWAITING_APPROVAL", "explicit approval of this digest required")
}

func (c *Controller) Approve(id, digest, actor string) (*Action, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, err := c.store.GetPatchAction(id)
	if err != nil {
		return nil, err
	}
	if a == nil || actor == "" || a.Owner != actor {
		return nil, errors.New("action not found")
	}
	if a.State != "AWAITING_APPROVAL" || digest != a.Digest || Digest(a.Parameters) != digest {
		return nil, errors.New("approval does not match pending action")
	}
	now := c.now().UnixMilli()
	if err := Validate(a.Parameters, now); err != nil {
		return nil, err
	}
	expires := now + MaxApprovalAgeMS
	if expires > a.Parameters.ExpiresAtMS {
		expires = a.Parameters.ExpiresAtMS
	}
	a.Approval = &Approval{actor, digest, now, expires}
	return a, c.event(a, "ARMED", "one-use human approval; output not yet executed")
}

func matches(p Parameters, d DeviceContext) error {
	if !d.Ready || !d.Connected {
		return errors.New("capability handshake missing or telemetry lost")
	}
	if p.Source != d.Source || p.DeviceID != d.DeviceID || p.BootID != d.BootID {
		return errors.New("source/device/boot mismatch")
	}
	if p.ProfileID != d.ProfileID || p.ProfileRevision != d.ProfileRevision || p.ProbeMapHash != d.ProbeMapHash {
		return errors.New("profile/revision/probe map mismatch")
	}
	if p.PatchPin != d.DedicatedPin || !d.AllowedNodes[p.TargetNode] {
		return errors.New("pin or target not allowlisted by hardware configuration")
	}
	if p.Source == "REAL_SERIAL" && !d.MasterEnabled {
		return errors.New("master physical enable is OFF")
	}
	return nil
}
func evidenceMatches(p Parameters, e Evidence) bool {
	return e.MeasurementID > 0 && e.Source == p.Source && e.DeviceID == p.DeviceID && e.BootID == p.BootID && e.ProfileID == p.ProfileID && e.ProfileRevision == p.ProfileRevision && e.ProbeMapHash == p.ProbeMapHash
}

// Run is only used with a trusted driver. No production serial output driver
// exists. The watchdog is supplemental: a future physical interface MUST also
// enforce a hardware/firmware deadline independent of this process.
func (c *Controller) Run(ctx context.Context, id string, d Driver) (action *Action, err error) {
	c.mu.Lock()
	defer c.mu.Unlock() // one output action at a time, including capture
	a, err := c.store.GetPatchAction(id)
	if err != nil || a == nil {
		return nil, errors.New("action not found")
	}
	p := a.Parameters
	now := c.now().UnixMilli()
	if a.State != "ARMED" || a.Approval == nil || a.Approval.Digest != Digest(p) || a.Digest != Digest(p) || now >= a.Approval.ExpiresAtMS {
		return a, errors.New("missing, changed, consumed or expired approval")
	}
	if err := Validate(p, now); err != nil {
		return a, err
	}
	if p.Source == "REAL_SERIAL" && !c.physicalVerified {
		return a, ErrLocked
	}
	if err := matches(p, d.Context()); err != nil {
		return a, err
	}
	// All failures after consuming approval end terminally; never retry output.
	attempted := false
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PATCH driver exception: %v", r)
		}
		if err != nil {
			if attempted {
				stopCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				stopErr := d.Disable(stopCtx, a.ID)
				cancel()
				if stopErr != nil {
					err = fmt.Errorf("%w; disable unconfirmed: %v", err, stopErr)
				}
			}
			a.Result = err.Error()
			if saveErr := c.event(a, "ABORTED", "no verified completion; "+err.Error()); saveErr != nil {
				err = fmt.Errorf("%w; audit: %v", err, saveErr)
			}
		}
		action = a
	}()
	before, err := d.Before(ctx)
	if err != nil {
		return a, err
	}
	if !evidenceMatches(p, before) {
		return a, errors.New("before evidence provenance mismatch")
	}
	a.Before = &before
	// Recheck after baseline collection, immediately before dispatch.
	if c.now().UnixMilli() >= a.Approval.ExpiresAtMS {
		return a, errors.New("approval expired before dispatch")
	}
	if err := matches(p, d.Context()); err != nil {
		return a, err
	}
	if err := c.event(a, "EXECUTING", "dispatch attempted; not proof of physical execution"); err != nil {
		return a, err
	}
	execution, cancel := context.WithTimeout(ctx, time.Duration(p.DurationMS+100)*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	watchStopped := make(chan struct{})
	go func() {
		defer close(watchStopped)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-execution.Done():
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				_ = d.Disable(stopCtx, a.ID)
				stopCancel()
				return
			case <-ticker.C:
				if matches(p, d.Context()) != nil {
					cancel()
				}
			}
		}
	}()
	attempted = true
	var receipt Receipt
	func() { defer func() { close(done); <-watchStopped }(); receipt, err = d.Execute(execution, *a) }()
	if err != nil {
		return a, err
	}
	if execution.Err() != nil {
		return a, execution.Err()
	}
	if err = matches(p, d.Context()); err != nil {
		return a, err
	}
	if receipt.ActionID != a.ID || receipt.BootID != p.BootID || !receipt.Disabled || receipt.DisabledAtMS <= 0 {
		return a, errors.New("no matching output-disabled acknowledgement")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = d.Disable(stopCtx, a.ID)
	stopCancel()
	if err != nil {
		return a, err
	}
	if err = c.event(a, "OUTPUT_DISABLED", "driver acknowledged output disabled"); err != nil {
		return a, err
	}
	if err = c.event(a, "RE_MEASURE", "waiting for a complete fresh post-disable window"); err != nil {
		return a, err
	}
	captureCtx, captureCancel := context.WithTimeout(ctx, 5*time.Second)
	defer captureCancel()
	after, err := d.CaptureAfter(captureCtx, receipt)
	if err != nil {
		return a, err
	}
	if !evidenceMatches(p, after) || after.MeasurementID == before.MeasurementID || after.Sequence <= before.Sequence || after.WindowStartedAtMS <= receipt.DisabledAtMS {
		return a, errors.New("post-action evidence is stale or provenance mismatched")
	}
	a.After = &after
	result, err := d.Verify(captureCtx, before, after)
	if err != nil {
		return a, err
	}
	if result == "" {
		return a, errors.New("verification returned no result")
	}
	a.Result = result
	return a, c.event(a, "VERIFY", "fresh evidence evaluated; see result (not necessarily repaired)")
}
