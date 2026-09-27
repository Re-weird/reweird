package patchcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

type memory struct{ actions map[string]Action }

func (m *memory) SavePatchAction(a Action) error {
	b, _ := json.Marshal(a)
	var copy Action
	_ = json.Unmarshal(b, &copy)
	m.actions[a.ID] = copy
	return nil
}
func (m *memory) GetPatchAction(id string) (*Action, error) {
	a, ok := m.actions[id]
	if !ok {
		return nil, nil
	}
	b, _ := json.Marshal(a)
	var copy Action
	_ = json.Unmarshal(b, &copy)
	return &copy, nil
}
func (m *memory) ListPatchActions(string) ([]Action, error) { return nil, nil }
func (m *memory) InterruptPatchActions(now int64) error {
	for id, a := range m.actions {
		a.State = "ABORTED"
		m.actions[id] = a
	}
	return nil
}

type fakeDriver struct {
	mu                          sync.Mutex
	device                      DeviceContext
	p                           Parameters
	disabled                    bool
	executions                  int
	loss, hang, stale, panicRun bool
	receipt                     Receipt
}

func (d *fakeDriver) Context() DeviceContext { d.mu.Lock(); defer d.mu.Unlock(); return d.device }
func (d *fakeDriver) Before(context.Context) (Evidence, error) {
	return Evidence{1, d.p.Source, d.p.DeviceID, d.p.BootID, d.p.ProfileID, d.p.ProfileRevision, d.p.ProbeMapHash, 1, 1}, nil
}
func (d *fakeDriver) Execute(ctx context.Context, a Action) (Receipt, error) {
	d.mu.Lock()
	d.executions++
	d.disabled = false
	if d.loss {
		d.device.Connected = false
	}
	d.mu.Unlock()
	if d.panicRun {
		panic("driver failed")
	}
	if d.hang || d.loss {
		<-ctx.Done()
		return Receipt{}, ctx.Err()
	}
	timer := time.NewTimer(time.Duration(a.Parameters.DurationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Receipt{}, ctx.Err()
	case <-timer.C:
	}
	d.mu.Lock()
	d.disabled = true
	d.mu.Unlock()
	d.receipt = Receipt{a.ID, a.Parameters.BootID, true, time.Now().UnixMilli()}
	return d.receipt, nil
}
func (d *fakeDriver) Disable(context.Context, string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.disabled = true
	return nil
}
func (d *fakeDriver) CaptureAfter(context.Context, Receipt) (Evidence, error) {
	if !d.disabled {
		return Evidence{}, errors.New("captured while output enabled")
	}
	e, _ := d.Before(context.Background())
	e.MeasurementID = 2
	e.Sequence = 2
	e.WindowStartedAtMS = d.receipt.DisabledAtMS + 1
	if d.stale {
		e.Sequence = 1
	}
	return e, nil
}
func (d *fakeDriver) Verify(context.Context, Evidence, Evidence) (string, error) {
	return "SIMULATED: evidence comparison completed; not a physical repair", nil
}
func fixture(t *testing.T) (*Controller, *memory, Parameters, *fakeDriver) {
	t.Helper()
	m := &memory{actions: map[string]Action{}}
	c, err := New(m)
	if err != nil {
		t.Fatal(err)
	}
	p := Parameters{"project", 1, "map", "device", "boot", "isolated-test-node", 10, "PULSE", "HIGH", 3.3, 0, 0, 1, time.Now().UnixMilli() + 30000, "SIMULATED"}
	d := &fakeDriver{p: p, disabled: true, device: DeviceContext{true, true, false, p.Source, p.DeviceID, p.BootID, p.ProfileID, p.ProbeMapHash, 1, 10, map[string]bool{p.TargetNode: true}}}
	return c, m, p, d
}
func armed(t *testing.T, c *Controller, p Parameters) *Action {
	t.Helper()
	a, err := c.Propose(p, "human")
	if err != nil {
		t.Fatal(err)
	}
	a, err = c.Approve(a.ID, a.Digest, "human")
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestValidation(t *testing.T) {
	_, _, p, _ := fixture(t)
	for name, change := range map[string]func(*Parameters){"duration": func(p *Parameters) { p.DurationMS = 251 }, "pin": func(p *Parameters) { p.PatchPin = 8 }, "voltage": func(p *Parameters) { p.MaxVoltage = 5 }, "level": func(p *Parameters) { p.LogicLevel = "5V" }, "mode": func(p *Parameters) { p.Mode = "SET_GPIO" }, "expired": func(p *Parameters) { p.ExpiresAtMS = 1 }, "frequency": func(p *Parameters) { p.FrequencyHz = 40000 }, "missing profile": func(p *Parameters) { p.ProfileID = "" }} {
		t.Run(name, func(t *testing.T) {
			q := p
			change(&q)
			if Validate(q, time.Now().UnixMilli()) == nil {
				t.Fatal("unsafe parameters accepted")
			}
		})
	}
}
func TestApprovedBoundedActionAndReplay(t *testing.T) {
	c, _, p, d := fixture(t)
	a := armed(t, c, p)
	a, err := c.Run(context.Background(), a.ID, d)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != "VERIFY" || !d.disabled || a.After == nil || a.After.Source != "SIMULATED" {
		t.Fatalf("bad completion: %+v", a)
	}
	expected := []string{"PROPOSED", "VALIDATED", "AWAITING_APPROVAL", "ARMED", "EXECUTING", "OUTPUT_DISABLED", "RE_MEASURE", "VERIFY"}
	if len(a.Events) != len(expected) {
		t.Fatal(a.Events)
	}
	for i, e := range a.Events {
		if e.State != expected[i] {
			t.Fatal(a.Events)
		}
	}
	if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 1 {
		t.Fatal("replayed action")
	}
}
func TestNoApproval(t *testing.T) {
	c, _, p, d := fixture(t)
	a, _ := c.Propose(p, "human")
	if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 0 {
		t.Fatal("executed without approval")
	}
}
func TestApprovalBinding(t *testing.T) {
	for _, scenario := range []string{"expired", "changed", "actor", "digest"} {
		t.Run(scenario, func(t *testing.T) {
			c, m, p, d := fixture(t)
			a := armed(t, c, p)
			switch scenario {
			case "expired":
				c.now = func() time.Time { return time.Now().Add(16 * time.Second) }
			case "changed":
				a.Parameters.DurationMS = 2
				_ = m.SavePatchAction(*a)
			case "actor":
				if _, err := c.Approve(a.ID, a.Digest, "someone-else"); err == nil {
					t.Fatal("wrong actor")
				}
				return
			case "digest":
				a, _ = c.Propose(p, "human")
				if _, err := c.Approve(a.ID, "changed", "human"); err == nil {
					t.Fatal("wrong digest")
				}
				return
			}
			if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 0 {
				t.Fatal("invalid approval executed")
			}
		})
	}
}
func TestContextMismatch(t *testing.T) {
	for _, scenario := range []string{"device", "profile", "reboot", "map", "capability", "simulator", "pin", "target", "disconnected"} {
		t.Run(scenario, func(t *testing.T) {
			c, _, p, d := fixture(t)
			a := armed(t, c, p)
			switch scenario {
			case "device":
				d.device.DeviceID = "other"
			case "profile":
				d.device.ProfileID = "other"
			case "reboot":
				d.device.BootID = "new-boot"
			case "map":
				d.device.ProbeMapHash = "changed"
			case "capability":
				d.device.Ready = false
			case "simulator":
				d.device.Source = "REAL_SERIAL"
			case "pin":
				d.device.DedicatedPin = 11
			case "target":
				d.device.AllowedNodes = map[string]bool{}
			case "disconnected":
				d.device.Connected = false
			}
			if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 0 {
				t.Fatal("invalid context executed")
			}
		})
	}
}
func TestLossTimeoutExceptionAndStaleEvidence(t *testing.T) {
	for _, scenario := range []string{"loss", "timeout", "exception", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			c, _, p, d := fixture(t)
			a := armed(t, c, p)
			switch scenario {
			case "loss":
				d.loss = true
			case "timeout":
				d.hang = true
			case "exception":
				d.panicRun = true
			case "stale":
				d.stale = true
			}
			a, err := c.Run(context.Background(), a.ID, d)
			if err == nil || !d.disabled || a.State != "ABORTED" {
				t.Fatalf("failed to disable/abort: %+v %v", a, err)
			}
		})
	}
}
func TestRestartInvalidatesApproval(t *testing.T) {
	c, m, p, d := fixture(t)
	a := armed(t, c, p)
	c, err := New(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 0 {
		t.Fatal("restart resumed output")
	}
}
func TestPhysicalAlwaysLocked(t *testing.T) {
	c, _, p, d := fixture(t)
	p.Source = "REAL_SERIAL"
	a, err := c.Propose(p, "human")
	if !errors.Is(err, ErrLocked) || a.State != "LOCKED" {
		t.Fatal(a, err)
	}
	d.device.MasterEnabled = true
	if _, err := c.Approve(a.ID, a.Digest, "human"); err == nil {
		t.Fatal("physical approved")
	}
	if _, err := c.Run(context.Background(), a.ID, d); err == nil || d.executions != 0 {
		t.Fatal("physical executed")
	}
}
