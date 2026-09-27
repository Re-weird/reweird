package store

import (
	"encoding/json"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"path/filepath"
	"testing"
	"time"
)

func TestBridgeRestartPreservesOriginalWireAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := physicalfixture.Frame(1)
	b := domain.BridgeBinding{ProjectID: "cloud-project", DeviceID: f.DeviceID, WireProfileID: f.ProfileID, TokenHash: "hash-not-token", ShareHash: "share-hash", ExpiresAtMS: time.Now().Add(time.Hour).UnixMilli(), Raw: &f, SentAtMS: 100, ProfileHash: "mapping-hash"}
	if err = s.SaveBridge(b); err != nil {
		t.Fatal(err)
	}
	b.Raw = nil
	b.TokenHash = "rotated-hash"
	if err = s.SaveBridge(b); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetBridge(b.ProjectID)
	if err != nil || got.TokenHash != "rotated-hash" {
		t.Fatal(got, err)
	}
	var raw string
	if err = s.db.QueryRow(`SELECT raw_payload FROM bridge_capture_audit WHERE project_id=?`, b.ProjectID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var original domain.TelemetryEnvelope
	if err = json.Unmarshal([]byte(raw), &original); err != nil || original.ProfileID != physicalfixture.ProfileID || original.Sequence != 1 {
		t.Fatal("original wire evidence lost", original, err)
	}
}
