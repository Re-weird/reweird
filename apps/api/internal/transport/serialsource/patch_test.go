package serialsource

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"net"
	"testing"
	"time"
)

func TestPatchReplyDemultiplexingPreservesTelemetry(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	s := New(a)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	go func() {
		scanner := bufio.NewScanner(b)
		if scanner.Scan() {
			var c patchcontrol.Command
			_ = json.Unmarshal(scanner.Bytes(), &c)
			r := patchcontrol.Reply{Type: "patch_status", RequestID: c.RequestID, OK: true, State: "LOCKED"}
			data, _ := json.Marshal(r)
			_, _ = b.Write(append(data, '\n'))
			_, _ = b.Write([]byte(`{"schema_version":2,"device_id":"board","profile_id":"project","window_ms":1000,"sequence":1,"samples":[{"probe":"P1","mode":"analog","analog_mv":[1650]}]}` + "\n"))
		}
	}()
	r, err := s.PatchExchange(ctx, patchcontrol.Command{Op: "status"})
	if err != nil || r.State != "LOCKED" {
		t.Fatal(r, err)
	}
	wait, done := context.WithTimeout(ctx, time.Second)
	defer done()
	f, err := s.WaitNext(wait, 0)
	if err != nil || f.DeviceID != "board" {
		t.Fatal(f, err)
	}
	if s.patchReply([]byte(`{"type":"telemetry","command":"drive"}`)) {
		t.Fatal("unknown data accepted as control")
	}
}
