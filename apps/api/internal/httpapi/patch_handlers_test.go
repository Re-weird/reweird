package httpapi

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

func TestPatchHTTPNeverUnlocksFromTelemetryOrEnableFlag(t *testing.T) {
	t.Setenv("REWEIRD_PATCH_ENABLE", "true") // cannot bypass qualification
	repository, err := store.Open(filepath.Join(t.TempDir(), "patch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	profile := physicalfixture.Profile()
	project := domain.Project{ID: profile.ID, OwnerID: "human", Name: "bench"}
	if err := repository.SaveProjectProfile(project, profile); err != nil {
		t.Fatal(err)
	}
	actor := "human"
	app := newApp(diagnostics.NewEngine(signalanalysis.New()), repository, fixedSerialSource{physicalfixture.Frame(1)}, profile.ID, ProjectServices{}, func(ctx *fiber.Ctx) error { ctx.Locals(ownerIDLocalsKey, actor); return ctx.Next() })
	var status struct {
		State   string `json:"state"`
		Enabled bool   `json:"physical_enabled"`
	}
	decodeBody(t, doJSON(t, app, "GET", "/api/v1/patch/status", nil), &status)
	if status.State != "PATCH LOCKED" || status.Enabled {
		t.Fatal(status)
	}
	if r := doJSON(t, app, "POST", "/api/v1/patch", map[string]any{"approved": true, "gpio": 8}); r.StatusCode != 423 {
		t.Fatal(r.StatusCode)
	}
	path := "/api/v1/projects/" + profile.ID + "/patch"
	p := patchcontrol.Parameters{ProfileID: profile.ID, ProfileRevision: profile.Version, ProbeMapHash: "map", DeviceID: "device", BootID: "boot", TargetNode: "isolated-test-node", PatchPin: 10, Mode: "PULSE", LogicLevel: "HIGH", MaxVoltage: 3.3, DurationMS: 10, ExpiresAtMS: time.Now().UnixMilli() + 30000, Source: "REAL_SERIAL"}
	r := doJSON(t, app, "POST", path+"/proposals", p)
	if r.StatusCode != 423 {
		t.Fatal(r.StatusCode)
	}
	var response struct {
		Action patchcontrol.Action `json:"action"`
	}
	decodeBody(t, r, &response)
	if response.Action.State != "LOCKED" || response.Action.Owner != "human" {
		t.Fatal(response)
	}
	if r := doJSON(t, app, "POST", path+"/actions/"+response.Action.ID+"/approve", map[string]any{"digest": response.Action.Digest}); r.StatusCode != 423 {
		t.Fatal(r.StatusCode)
	}
	p.Source = "SIMULATED"
	if r := doJSON(t, app, "POST", path+"/proposals", p); r.StatusCode != 400 {
		t.Fatal("simulator physical proposal accepted")
	}
	if r := doJSON(t, app, "POST", path+"/proposals", map[string]any{"approved": true, "executed": true}); r.StatusCode != 400 {
		t.Fatal("client execution claims accepted")
	}
	actor = "other"
	if r := doJSON(t, app, "GET", path+"/actions", nil); r.StatusCode != http.StatusNotFound {
		t.Fatal("cross-owner audit access")
	}
	actor = ""
	if r := doJSON(t, app, "POST", path+"/proposals", p); r.StatusCode != http.StatusUnauthorized {
		t.Fatal("anonymous action accepted")
	}
}
