package patchcontrol

import (
	"context"
	"errors"
	"testing"
)

type wireFunc func(context.Context, Command) (Reply, error)

func (f wireFunc) PatchExchange(c context.Context, p Command) (Reply, error) { return f(c, p) }
func TestWireDriverRejectsUnverifiedCompletion(t *testing.T) {
	for _, mode := range []string{"changed_boot", "changed_action", "lease_expired", "unknown_state", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			c, _, p, _ := fixture(t)
			a := armed(t, c, p)
			capability := Reply{OK: true, State: "READY", DeviceID: p.DeviceID, BootID: p.BootID, Challenge: "01234567890123456789012345678901"}
			d := &WireDriver{Capability: capability, Transport: wireFunc(func(ctx context.Context, cmd Command) (Reply, error) {
				r := capability
				r.ActionID = a.ID
				r.Digest = a.Digest
				if cmd.Op == "arm" {
					r.State = "ARMED"
					return r, nil
				}
				r.State = "DISABLED"
				r.Completed = true
				switch mode {
				case "changed_boot":
					r.BootID = "reboot"
				case "changed_action":
					r.ActionID = "other"
				case "lease_expired":
					r.Completed = false
				case "unknown_state":
					r.State = "UNKNOWN"
				case "timeout":
					return r, errors.New("timeout")
				}
				return r, nil
			})}
			if _, e := d.Execute(context.Background(), *a); e == nil {
				t.Fatal("unverified completion accepted")
			}
		})
	}
}
func TestCancellationAndOLEDProtection(t *testing.T) {
	c, _, p, d := fixture(t)
	a := armed(t, c, p)
	if _, e := c.Cancel(a.ID, "human"); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Run(context.Background(), a.ID, d); e == nil || d.executions != 0 {
		t.Fatal("cancelled action executed")
	}
	for _, pin := range []int{4, 5} {
		p.PatchPin = pin
		if Validate(p, c.now().UnixMilli()) == nil {
			t.Fatal("OLED pin accepted", pin)
		}
	}
}
