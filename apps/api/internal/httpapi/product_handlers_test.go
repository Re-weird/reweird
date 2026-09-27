package httpapi

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/productdata"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/telemetrystore"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

// testProductApp mirrors testOwnedApp (ownership_test.go) but wires a real
// in-memory ProductRepository and the real Component Catalog, so equipment/
// me/catalog handlers can be exercised end-to-end without a live MongoDB
// deployment.
func testProductApp(t *testing.T) *fiber.App {
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
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	fakeOwnerMiddleware := func(ctx *fiber.Ctx) error {
		if owner := ctx.Get("X-Test-Owner"); owner != "" {
			// strings.Clone forces a fresh allocation: fasthttp reuses its
			// internal header-parsing buffer across requests on a pooled
			// ctx, so an uncloned string read here would silently change
			// value once a later request overwrites that buffer -- fatal
			// for a store like ours that keeps the struct in memory
			// in-process without a serialize/deserialize boundary to force
			// a copy (unlike the SQLite-backed store, which is why this
			// exact pattern in ownership_test.go's fakeOwnerMiddleware
			// never surfaced it).
			ctx.Locals(ownerIDLocalsKey, strings.Clone(owner))
		}
		return ctx.Next()
	}
	services := ProjectServices{Product: productdata.NewMemoryStore(), Catalog: catalog, Telemetry: telemetrystore.NewMemoryStore()}
	return newApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, services, fakeOwnerMiddleware)
}

func TestEquipmentRequiresAuthentication(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", "", map[string]any{"name": "Front sensor"})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("create equipment anonymously status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	response = doJSONAs(t, app, http.MethodGet, "/api/v1/equipment", "", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("list equipment anonymously status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestEquipmentOwnerCanCreateListAndRead(t *testing.T) {
	app := testProductApp(t)

	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", "owner-a", map[string]any{"name": "Front Distance Sensor", "catalog_id": "hc-sr04"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create equipment status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var created struct {
		ID        string  `json:"id"`
		OwnerID   string  `json:"owner_id"`
		CatalogID *string `json:"catalog_id"`
	}
	decodeBody(t, response, &created)
	if created.OwnerID != "owner-a" {
		t.Fatalf("OwnerID = %q, want %q (browser must never set its own owner_id)", created.OwnerID, "owner-a")
	}
	if created.CatalogID == nil || *created.CatalogID != "hc-sr04" {
		t.Fatalf("CatalogID = %v, want hc-sr04", created.CatalogID)
	}

	listResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/equipment", "owner-a", nil)
	var list []map[string]any
	decodeBody(t, listResponse, &list)
	if len(list) != 1 {
		t.Fatalf("list = %v, want exactly one item", list)
	}

	getResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/equipment/"+created.ID, "owner-a", nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get equipment status = %d body=%s", getResponse.StatusCode, readBody(t, getResponse))
	}
}

func TestEquipmentCustomWithoutCatalogIDIsAllowed(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", "owner-a", map[string]any{"name": "Custom Motor Controller"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create custom equipment status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestEquipmentInvalidCatalogIDIsRejected(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", "owner-a", map[string]any{"name": "Mystery Part", "catalog_id": "does-not-exist"})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("create equipment with bad catalog_id status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestEquipmentOwnershipIsolationAcrossUsers(t *testing.T) {
	app := testProductApp(t)
	created := createEquipmentAs(t, app, "owner-a", "Front sensor")

	// User B must never read, update, or delete User A's equipment, and
	// must see an identical 404 to "this id does not exist" -- never a 403
	// that would confirm the id belongs to someone else.
	if response := doJSONAs(t, app, http.MethodGet, "/api/v1/equipment/"+created.ID, "owner-b", nil); response.StatusCode != http.StatusNotFound {
		t.Fatalf("get as other owner status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	if response := doJSONAs(t, app, http.MethodPatch, "/api/v1/equipment/"+created.ID, "owner-b", map[string]any{"name": "Hijacked"}); response.StatusCode != http.StatusNotFound {
		t.Fatalf("update as other owner status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	if response := doJSONAs(t, app, http.MethodDelete, "/api/v1/equipment/"+created.ID, "owner-b", nil); response.StatusCode != http.StatusNotFound {
		t.Fatalf("delete as other owner status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	list := listEquipmentAs(t, app, "owner-b")
	if len(list) != 0 {
		t.Fatalf("list as other owner = %v, want empty (no enumeration of another owner's equipment)", list)
	}
}

func TestEquipmentBrowserSuppliedOwnerIDCannotStealOwnership(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", "owner-a", map[string]any{"name": "Sensor", "owner_id": "owner-b"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create equipment status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var created struct {
		OwnerID string `json:"owner_id"`
	}
	decodeBody(t, response, &created)
	if created.OwnerID != "owner-a" {
		t.Fatalf("OwnerID = %q, want the verified caller %q regardless of a client-supplied owner_id field", created.OwnerID, "owner-a")
	}
}

func TestEquipmentDeleteRemovesIt(t *testing.T) {
	app := testProductApp(t)
	created := createEquipmentAs(t, app, "owner-a", "Sensor")
	if response := doJSONAs(t, app, http.MethodDelete, "/api/v1/equipment/"+created.ID, "owner-a", nil); response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	if response := doJSONAs(t, app, http.MethodGet, "/api/v1/equipment/"+created.ID, "owner-a", nil); response.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete status = %d", response.StatusCode)
	}
}

func TestProjectEquipmentAttachRequiresOwningBothSides(t *testing.T) {
	app := testProductApp(t)
	equipmentA := createEquipmentAs(t, app, "owner-a", "Sensor A")

	project := createProjectAs(t, app, "owner-a", "Robot Car")

	// User A attaching their own equipment to their own project succeeds.
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/equipment", "owner-a", map[string]any{"equipment_id": equipmentA.ID, "role": "distance sensing"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("attach status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	// Duplicate attachment is rejected.
	response = doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/equipment", "owner-a", map[string]any{"equipment_id": equipmentA.ID, "role": "distance sensing"})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate attach status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	// User B cannot attach User A's equipment to User B's own project, even
	// though User B does own that project.
	projectB := createProjectAs(t, app, "owner-b", "Someone Else's Project")
	response = doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectB.ID+"/equipment", "owner-b", map[string]any{"equipment_id": equipmentA.ID, "role": "distance sensing"})
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("attach other owner's equipment status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	// User B cannot attach anything to User A's project.
	equipmentB := createEquipmentAs(t, app, "owner-b", "Sensor B")
	response = doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/equipment", "owner-b", map[string]any{"equipment_id": equipmentB.ID, "role": "distance sensing"})
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("attach to other owner's project status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	list := listProjectEquipmentAs(t, app, "owner-a", project.ID)
	if len(list) != 1 {
		t.Fatalf("list project equipment = %v, want exactly one link", list)
	}

	detach := doJSONAs(t, app, http.MethodDelete, "/api/v1/projects/"+project.ID+"/equipment/"+equipmentA.ID, "owner-a", nil)
	if detach.StatusCode != http.StatusNoContent {
		t.Fatalf("detach status = %d body=%s", detach.StatusCode, readBody(t, detach))
	}
	if len(listProjectEquipmentAs(t, app, "owner-a", project.ID)) != 0 {
		t.Fatalf("expected no links after detach")
	}
}

func TestDeleteEquipmentClearsProjectAttachments(t *testing.T) {
	app := testProductApp(t)
	equipment := createEquipmentAs(t, app, "owner-a", "Sensor")
	project := createProjectAs(t, app, "owner-a", "Robot Car")
	doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/equipment", "owner-a", map[string]any{"equipment_id": equipment.ID})

	if response := doJSONAs(t, app, http.MethodDelete, "/api/v1/equipment/"+equipment.ID, "owner-a", nil); response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete equipment status = %d", response.StatusCode)
	}
	if len(listProjectEquipmentAs(t, app, "owner-a", project.ID)) != 0 {
		t.Fatalf("expected the attachment to be removed alongside the deleted equipment")
	}
}

func TestMeReturnsAndPersistsVerifiedIdentity(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodGet, "/api/v1/me", "owner-a", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /me status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var user struct {
		ID          string `json:"id"`
		AuthSubject string `json:"auth_subject"`
	}
	decodeBody(t, response, &user)
	if user.AuthSubject != "owner-a" {
		t.Fatalf("me.AuthSubject = %q, want %q", user.AuthSubject, "owner-a")
	}
	if user.ID == "" {
		t.Fatalf("me.ID = %q, want a generated user id", user.ID)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodGet, "/api/v1/me", "", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /me anonymously status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestCatalogReadOnlyEndpoints(t *testing.T) {
	app := testProductApp(t)
	response := doJSONAs(t, app, http.MethodGet, "/api/v1/catalog", "", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /catalog status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	entryResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/catalog/hc-sr04", "", nil)
	if entryResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /catalog/hc-sr04 status = %d body=%s", entryResponse.StatusCode, readBody(t, entryResponse))
	}
	missingResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/catalog/does-not-exist", "", nil)
	if missingResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /catalog/does-not-exist status = %d", missingResponse.StatusCode)
	}
}

// --- shared small helpers for this file ---

type testEquipment struct {
	ID string `json:"id"`
}

func createEquipmentAs(t *testing.T, app *fiber.App, owner, name string) testEquipment {
	t.Helper()
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/equipment", owner, map[string]any{"name": name})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create equipment as %q status = %d body=%s", owner, response.StatusCode, readBody(t, response))
	}
	var created testEquipment
	decodeBody(t, response, &created)
	return created
}

func listEquipmentAs(t *testing.T, app *fiber.App, owner string) []map[string]any {
	t.Helper()
	response := doJSONAs(t, app, http.MethodGet, "/api/v1/equipment", owner, nil)
	var list []map[string]any
	decodeBody(t, response, &list)
	return list
}

func listProjectEquipmentAs(t *testing.T, app *fiber.App, owner, projectID string) []map[string]any {
	t.Helper()
	response := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectID+"/equipment", owner, nil)
	var list []map[string]any
	decodeBody(t, response, &list)
	return list
}

type testProject struct {
	ID string `json:"id"`
}

func createProjectAs(t *testing.T, app *fiber.App, owner, name string) testProject {
	t.Helper()
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", owner, map[string]any{"name": name, "controller": "ESP32", "logic_voltage": 3.3})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create project as %q status = %d body=%s", owner, response.StatusCode, readBody(t, response))
	}
	var project testProject
	decodeBody(t, response, &project)
	return project
}
