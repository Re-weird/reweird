package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

func TestUSBBridgeCloudLifecycle(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "bridge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	p := physicalfixture.Profile()
	p.ID = "cloud-physical-project"
	p.ProjectID = p.ID
	project := domain.Project{ID: p.ID, OwnerID: "owner", Name: "Private circuit", ProbePlan: &domain.ProbePlan{ProfileID: p.ID, Connected: true, ConnectedAtMS: 1}}
	if err = repo.SaveProjectProfile(project, p); err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveProfile(profiles.UltrasonicDemo()); err != nil {
		t.Fatal(err)
	}
	makeApp := func() *fiber.App {
		return newApp(diagnostics.NewEngine(signalanalysis.New()), repo, simulator.NewUltrasonicSource(), "ultrasonic-demo", ProjectServices{}, func(ctx *fiber.Ctx) error { ctx.Locals(ownerIDLocalsKey, ctx.Get("Test-Owner")); return ctx.Next() })
	}
	app := makeApp()
	request := func(method, path, owner, header, token string, body any) (int, map[string]any) {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Test-Owner", owner)
		if header != "" {
			r.Header.Set(header, token)
		}
		res, err := app.Test(r, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var result map[string]any
		_ = json.Unmarshal(raw, &result)
		return res.StatusCode, result
	}
	pairPath := "/api/v1/projects/" + p.ID + "/bridge/pair"
	input := map[string]any{"device_id": physicalfixture.DeviceID, "wire_profile_id": physicalfixture.ProfileID, "confirm_mapping": true}
	if status, _ := request("POST", pairPath, "", "", "", input); status != 401 {
		t.Fatal("anonymous pairing", status)
	}
	if status, _ := request("POST", pairPath, "other", "", "", input); status != 404 {
		t.Fatal("cross owner pairing", status)
	}
	status, paired := request("POST", pairPath, "owner", "", "", input)
	if status != 200 {
		t.Fatal(status, paired)
	}
	token := paired["token"].(string)
	share := paired["share_token"].(string)
	b, _ := repo.GetBridge(p.ID)
	if b.TokenHash == token || b.ShareHash == share {
		t.Fatal("plaintext credentials stored")
	}
	path := "/api/v1/bridge/" + p.ID + "/frames"
	f := physicalfixture.Frame(10)
	f.UptimeMS = 10000
	packet := func(frame domain.TelemetryEnvelope, at int64) any {
		return map[string]any{"sent_at_ms": at, "frame": frame}
	}
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", share, packet(f, time.Now().UnixMilli())); status != 401 {
		t.Fatal("judge token allowed ingress", status)
	}
	bad := f
	bad.DeviceID = "wrong-device"
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(bad, time.Now().UnixMilli())); status != 409 {
		t.Fatal("wrong device", status)
	}
	bad = f
	bad.ProfileID = "wrong-profile"
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(bad, time.Now().UnixMilli())); status != 409 {
		t.Fatal("wrong profile", status)
	}
	bad = f
	bad.Samples = append([]domain.TelemetrySample(nil), f.Samples...)
	bad.Samples[4].Mode = domain.ProbeModeDigital
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(bad, time.Now().UnixMilli())); status != 422 {
		t.Fatal("P5 mismatch", status)
	}
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(f, time.Now().Add(-time.Minute).UnixMilli())); status != 422 {
		t.Fatal("stale frame", status)
	}
	status, result := request("POST", path, "", "X-ReWeird-Bridge", token, packet(f, time.Now().UnixMilli()))
	if status != 200 {
		t.Fatal(status, result)
	}
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(f, time.Now().UnixMilli())); status != 409 {
		t.Fatal("replay", status)
	}
	status, result = request("GET", "/api/v1/session?project_id="+p.ID, "owner", "", "", nil)
	if status != 200 || result["telemetry_mode"] != "serial" || result["profile_id"] != p.ID {
		t.Fatal("cloud workbench", status, result)
	}
	b, _ = repo.GetBridge(p.ID)
	if b.Raw.ProfileID != physicalfixture.ProfileID {
		t.Fatal("original wire evidence lost")
	}
	if status, _ = request("GET", "/api/v1/session?project_id="+p.ID, "other", "", "", nil); status != 404 {
		t.Fatal("cross project read", status)
	}
	status, result = request("GET", "/api/v1/session", "", "", "", nil)
	if status != 200 || result["telemetry_mode"] != "simulator" {
		t.Fatal("simulator overwritten", status, result)
	}
	viewPath := "/api/v1/bridge/" + p.ID + "/view"
	status, result = request("GET", viewPath, "", "X-ReWeird-Share", share, nil)
	if status != 200 || result["connected"] != true || result["patch"] != "LOCKED" {
		t.Fatal("judge view", status, result)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "token") || result["raw_telemetry"] != nil || result["owner_id"] != nil {
		t.Fatal("private data exposed")
	}
	if status, _ = request("POST", pairPath, "", "X-ReWeird-Share", share, input); status != 401 {
		t.Fatal("share used as owner")
	}
	if status, _ = request("GET", "/api/v1/measurements?profile_id="+p.ID, "", "X-ReWeird-Share", share, nil); status != 404 {
		t.Fatal("share exposed private measurements", status)
	}
	if status, _ = request("GET", "/api/v1/profiles/"+p.ID, "", "X-ReWeird-Share", share, nil); status != 404 {
		t.Fatal("judge accessed private profile", status)
	}
	privateTestID := "test-01234567890123456789012345678901"
	if err = repo.SaveTestWorkflow(domain.DiagnosticWorkflow{ID: privateTestID, ProjectID: p.ID, ProfileID: p.ID, Status: domain.TestPlanned}); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/api/v1/history/", "/api/v1/reports/", "/api/v1/tests/"} {
		if status, _ = request("GET", route+privateTestID, "", "X-ReWeird-Share", share, nil); status != 404 {
			t.Fatal("judge accessed private workflow", route, status)
		}
	}
	if status, _ = request("POST", "/api/v1/profiles/"+p.ID+"/known-good", "", "X-ReWeird-Share", share, map[string]any{"measurement_id": 1, "confirm_healthy": true}); status != 404 {
		t.Fatal("judge could confirm Known Good", status)
	}
	// Capture every window without any Workbench polling between uploads.
	for sequence := uint64(11); sequence <= 19; sequence++ {
		b, _ = repo.GetBridge(p.ID)
		b.ReceivedAtMS = time.Now().Add(-time.Second).UnixMilli() // advance rate limiter clock in fixture
		if err = repo.SaveBridge(*b); err != nil {
			t.Fatal(err)
		}
		next := physicalfixture.Frame(sequence)
		next.UptimeMS = sequence * 1000
		at := time.Now().UnixMilli()
		if at <= b.SentAtMS {
			at = b.SentAtMS + 1
		}
		status, result = request("POST", path, "", "X-ReWeird-Bridge", token, packet(next, at))
		if status != 200 {
			t.Fatal("consecutive capture", sequence, status, result)
		}
	}
	windows, err := repo.ListMeasurements(p.ID, 20)
	if err != nil || len(windows) != 10 {
		t.Fatal("calibration lost unpolled windows", len(windows), err)
	}
	status, result = request("GET", "/api/v1/profiles/"+p.ID+"/calibration", "owner", "", "", nil)
	if status != 200 {
		t.Fatal("cloud calibration", status, result)
	}
	// A profile edit invalidates live use, without altering saved Known Good.
	changed := p
	changed.Version++
	if err = repo.SaveProfile(changed); err != nil {
		t.Fatal(err)
	}
	status, result = request("GET", viewPath, "", "X-ReWeird-Share", share, nil)
	if status != 200 || result["connected"] != false {
		t.Fatal("profile edit allowed live", status, result)
	}
	if err = repo.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	// Restart retains credentials and replay protection, not a simulator fallback.
	app = makeApp()
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(f, time.Now().UnixMilli())); status != 409 {
		t.Fatal("restart replay", status)
	}
	b, _ = repo.GetBridge(p.ID)
	b.ReceivedAtMS = time.Now().Add(-10 * time.Second).UnixMilli()
	if err = repo.SaveBridge(*b); err != nil {
		t.Fatal(err)
	}
	status, result = request("GET", viewPath, "", "X-ReWeird-Share", share, nil)
	if status != 200 || result["connected"] != false || result["probes"] != nil {
		t.Fatal("offline stale data", status, result)
	}
	status, result = request("GET", "/api/v1/telemetry/status?project_id="+p.ID, "owner", "", "", nil)
	if status != 503 || result["mode"] != "serial" {
		t.Fatal("offline simulator fallback", status, result)
	}
	if status, _ = request("DELETE", "/api/v1/projects/"+p.ID+"/bridge", "owner", "", "", nil); status != 204 {
		t.Fatal("revoke", status)
	}
	if status, _ = request("GET", viewPath, "", "X-ReWeird-Share", share, nil); status != 401 {
		t.Fatal("revoked share")
	}
	if status, _ = request("POST", path, "", "X-ReWeird-Bridge", token, packet(f, time.Now().UnixMilli())); status != 401 {
		t.Fatal("revoked bridge")
	}
}
