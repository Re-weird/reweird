package store

import (
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"path/filepath"
	"testing"
	"time"
)

func TestPatchAuditSurvivesRestartWithoutRearming(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patch.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := patchcontrol.New(s)
	if err != nil {
		t.Fatal(err)
	}
	p := patchcontrol.Parameters{ProfileID: "bench", ProfileRevision: 1, ProbeMapHash: "map", DeviceID: "virtual", BootID: "boot", TargetNode: "isolated-node", PatchPin: 10, Mode: "PULSE", LogicLevel: "HIGH", MaxVoltage: 3.3, DurationMS: 10, ExpiresAtMS: time.Now().UnixMilli() + 30000, Source: "SIMULATED"}
	a, err := c.Propose(p, "human")
	if err != nil {
		t.Fatal(err)
	}
	a, err = c.Approve(a.ID, a.Digest, "human")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := patchcontrol.New(s); err != nil {
		t.Fatal(err)
	}
	restored, err := s.GetPatchAction(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.State != "ABORTED" || restored.Approval.Actor != "human" || restored.Digest != a.Digest || len(restored.Events) != 5 {
		t.Fatalf("audit lost or restarted armed: %+v", restored)
	}
}
