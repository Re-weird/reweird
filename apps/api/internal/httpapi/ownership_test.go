package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

// testOwnedApp wires up a real app via the same newApp used in production,
// but with a fake owner-identity middleware that trusts a test-only
// "X-Test-Owner" header instead of verifying a real signed token. This lets
// the ownership isolation logic itself be tested deterministically, without
// needing a real AUTH_TOKEN_SECRET-signed token. Production never uses this
// path — NewApp always wires the real ownerContext(authConfigured).
func testOwnedApp(t *testing.T) *fiber.App {
	t.Helper()
	root := t.TempDir()
	repository, err := store.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	demo := profiles.UltrasonicDemo()
	if err := repository.SaveProfile(demo); err != nil {
		t.Fatal(err)
	}
	fakeOwnerMiddleware := func(ctx *fiber.Ctx) error {
		if owner := ctx.Get("X-Test-Owner"); owner != "" {
			ctx.Locals(ownerIDLocalsKey, owner)
		}
		return ctx.Next()
	}
	return newApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, ProjectServices{UploadRoot: filepath.Join(root, "uploads")}, fakeOwnerMiddleware)
}

func doJSONAs(t *testing.T, app *fiber.App, method, path, owner string, payload any) *http.Response {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	if owner != "" {
		request.Header.Set("X-Test-Owner", owner)
	}
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestProjectOwnershipIsolation(t *testing.T) {
	app := testOwnedApp(t)

	createAs := func(owner, name string) domain.Project {
		response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", owner, map[string]any{"name": name, "controller": "ESP32", "logic_voltage": 3.3})
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create as %q status = %d body=%s", owner, response.StatusCode, readBody(t, response))
		}
		var project domain.Project
		decodeBody(t, response, &project)
		return project
	}

	anon := createAs("", "Shared Demo Project")
	ownedByA := createAs("user-a", "Alex's Robot")
	ownedByB := createAs("user-b", "Bailey's Robot")

	if ownedByA.OwnerID != "user-a" || ownedByB.OwnerID != "user-b" || anon.OwnerID != "" {
		t.Fatalf("owner stamping failed: anon=%q a=%q b=%q", anon.OwnerID, ownedByA.OwnerID, ownedByB.OwnerID)
	}

	// User A's list should show only their own project, not user B's or the anonymous one.
	listResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/projects", "user-a", nil)
	var listedForA []domain.Project
	decodeBody(t, listResponse, &listedForA)
	if len(listedForA) != 1 || listedForA[0].ID != ownedByA.ID {
		t.Fatalf("user-a's project list leaked or missing entries: %#v", listedForA)
	}

	// Anonymous callers should see only the anonymous/demo-pool project.
	listAnon := doJSONAs(t, app, http.MethodGet, "/api/v1/projects", "", nil)
	var listedAnon []domain.Project
	decodeBody(t, listAnon, &listedAnon)
	if len(listedAnon) != 1 || listedAnon[0].ID != anon.ID {
		t.Fatalf("anonymous project list leaked authenticated projects: %#v", listedAnon)
	}

	// User B must not be able to fetch user A's project by ID — not found, not forbidden (don't confirm existence).
	getAsB := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+ownedByA.ID, "user-b", nil)
	if getAsB.StatusCode != http.StatusNotFound {
		t.Fatalf("user-b reading user-a's project status = %d body=%s", getAsB.StatusCode, readBody(t, getAsB))
	}

	// An anonymous caller must not be able to fetch user A's project either.
	getAsAnon := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+ownedByA.ID, "", nil)
	if getAsAnon.StatusCode != http.StatusNotFound {
		t.Fatalf("anonymous reading user-a's project status = %d body=%s", getAsAnon.StatusCode, readBody(t, getAsAnon))
	}

	// User A can still read their own project.
	getAsA := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+ownedByA.ID, "user-a", nil)
	if getAsA.StatusCode != http.StatusOK {
		t.Fatalf("user-a reading their own project status = %d body=%s", getAsA.StatusCode, readBody(t, getAsA))
	}

	// A nested sub-resource route (code upload) inherits the same protection via findProject.
	codeAsB := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+ownedByA.ID+"/code", "user-b", map[string]any{"filename": "a.ino", "code_text": "void setup(){}"})
	if codeAsB.StatusCode != http.StatusNotFound {
		t.Fatalf("user-b uploading code to user-a's project status = %d body=%s", codeAsB.StatusCode, readBody(t, codeAsB))
	}
}

// TestOwnerContextRealMiddleware exercises the real ownerContext middleware
// (not the test double above) to confirm the three documented outcomes: no
// header -> anonymous, unconfigured -> anonymous even with a header present,
// and a malformed/invalid bearer token -> 401, never silently downgraded.
func TestOwnerContextRealMiddleware(t *testing.T) {
	newRealApp := func(t *testing.T, authConfigured bool) *fiber.App {
		t.Helper()
		root := t.TempDir()
		repository, err := store.Open(filepath.Join(root, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repository.Close() })
		demo := profiles.UltrasonicDemo()
		if err := repository.SaveProfile(demo); err != nil {
			t.Fatal(err)
		}
		return NewApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, ProjectServices{}, authConfigured)
	}

	t.Run("no header is anonymous", func(t *testing.T) {
		app := newRealApp(t, true)
		request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
		}
	})

	t.Run("unconfigured backend treats a presented token as anonymous", func(t *testing.T) {
		app := newRealApp(t, false)
		request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		request.Header.Set("Authorization", "Bearer not-a-real-token")
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
		}
	})

	t.Run("configured backend rejects an invalid token with 401, not anonymous fallback", func(t *testing.T) {
		app := newRealApp(t, true)
		request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		request.Header.Set("Authorization", "Bearer not-a-real-token")
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d body=%s, want 401", response.StatusCode, readBody(t, response))
		}
	})

	t.Run("configured backend rejects a non-Bearer scheme with 401", func(t *testing.T) {
		app := newRealApp(t, true)
		request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d body=%s, want 401", response.StatusCode, readBody(t, response))
		}
	})
}
