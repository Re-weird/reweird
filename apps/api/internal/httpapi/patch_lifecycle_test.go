package httpapi

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"path/filepath"
	"sync"
	"testing"
)

// Emulates only the wire; all HTTP authorization, driver, persisted capture and
// existing VERIFY code run unchanged. This is NOT physical qualification.
type qualifiedWire struct {
	mu      sync.Mutex
	profile domain.ProjectProfile
	reply   patchcontrol.Reply
	seq     uint64
	outputs int
	dead    bool
}

func (s *qualifiedWire) Name() string    { return "serial" }
func (s *qualifiedWire) PatchLive() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.dead }
func (s *qualifiedWire) Latest(context.Context) (domain.TelemetryEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := physicalfixture.Frame(s.seq)
	f.UptimeMS = uint64(1000 + s.seq*1000)
	f.Patch = &domain.PatchCapability{Capable: true, BootID: 1, State: s.reply.State, MaxDurationMS: 250}
	return f, nil
}
func (s *qualifiedWire) WaitNext(ctx context.Context, seq uint64) (domain.TelemetryEnvelope, error) {
	s.mu.Lock()
	s.seq++
	s.mu.Unlock()
	return s.Latest(ctx)
}
func (s *qualifiedWire) PatchExchange(ctx context.Context, c patchcontrol.Command) (patchcontrol.Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.reply
	r.Type = "patch_status"
	r.OK = true
	switch c.Op {
	case "hello":
		r.State = "DISABLED"
	case "arm":
		r.State = "ARMED"
		r.ActionID = c.ActionID
		r.Digest = c.Digest
	case "execute":
		r.State = "ACTIVE"
		s.outputs++
	case "poll":
		r.State = "DISABLED"
		r.Completed = true
		r.UptimeMS = 2000
	case "disable":
		r.State = "DISABLED"
	}
	s.reply = r
	return r, nil
}
func TestPatchHTTPFullQualifiedLifecycleAndRestart(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "patch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	p := physicalfixture.Profile()
	p.Connections = []domain.ProfileConnection{{ID: "trig", Probe: "P2", Target: "isolated-trigger-input", Confirmed: true}}
	project := domain.Project{ID: p.ID, OwnerID: "human", ProbePlan: &domain.ProbePlan{ProfileID: p.ID, Connected: true, ConnectedAtMS: 1}}
	if err := repo.SaveProjectProfile(project, p); err != nil {
		t.Fatal(err)
	}
	source := &qualifiedWire{profile: p, seq: 1, reply: patchcontrol.Reply{OK: true, State: "READY", Challenge: "01234567890123456789012345678901", QualificationID: "TEST-ONLY", DeviceID: physicalfixture.DeviceID, BootID: "1", ProfileID: p.ID, ProfileRevision: p.Version, ProbeMapHash: passport.MappingHash(p), Pin: 10, TargetNode: "isolated-trigger-input"}}
	makeApp := func() *fiber.App {
		return newApp(diagnostics.NewEngine(signalanalysis.New()), repo, source, p.ID, ProjectServices{}, func(ctx *fiber.Ctx) error { ctx.Locals(ownerIDLocalsKey, "human"); return ctx.Next() })
	}
	app := makeApp()
	path := "/api/v1/projects/" + p.ID + "/patch"
	var a patchcontrol.Action
	decodeBody(t, doJSON(t, app, "POST", path+"/prepare", map[string]any{"level": "HIGH", "duration_ms": 10}), &a)
	if a.State != "AWAITING_APPROVAL" || source.outputs != 0 {
		t.Fatalf("unsafe prepare: %+v", a)
	}
	approve := path + "/actions/" + a.ID + "/approve"
	if r := doJSON(t, app, "POST", approve, map[string]any{"digest": a.Digest, "confirm": true}); r.StatusCode != 423 {
		t.Fatal("master default not OFF", r.StatusCode)
	}
	if r := doJSON(t, app, "POST", path+"/master", map[string]any{"enabled": true, "confirm": true}); r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	if r := doJSON(t, app, "POST", approve, map[string]any{"digest": a.Digest, "confirm": false}); r.StatusCode != 400 {
		t.Fatal("missing approval accepted")
	}
	if r := doJSON(t, app, "POST", approve, map[string]any{"digest": "changed", "confirm": true}); r.StatusCode != 409 {
		t.Fatal("changed digest accepted")
	}
	var result patchcontrol.Action
	r := doJSON(t, app, "POST", approve, map[string]any{"digest": a.Digest, "confirm": true})
	decodeBody(t, r, &result)
	if r.StatusCode != 200 || result.State != "VERIFY" || result.Before == nil || result.After == nil || result.Before.MeasurementID == result.After.MeasurementID || result.After.Source != "REAL_SERIAL" || source.outputs != 1 {
		t.Fatalf("lifecycle failed: status=%d %+v", r.StatusCode, result)
	}
	if r := doJSON(t, app, "POST", approve, map[string]any{"digest": a.Digest, "confirm": true}); r.StatusCode != 409 {
		t.Fatal("replay accepted")
	}
	saved, err := repo.GetPatchAction(a.ID)
	if err != nil || saved.State != "VERIFY" || len(saved.Events) < 8 {
		t.Fatal("audit missing", err, saved)
	}
	app = makeApp()
	var status struct {
		Master bool `json:"master_enabled"`
	}
	decodeBody(t, doJSON(t, app, "GET", "/api/v1/patch/status", nil), &status)
	if status.Master {
		t.Fatal("master survived restart")
	}
}
