package httpapi

import (
	"context"
	"errors"
	"net/http"
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

// failingTelemetrySink always errors, so tests can prove a Tiger Data
// failure never breaks the request that triggered it -- SQLite has already
// durably saved the measurement by the time Insert is attempted.
type failingTelemetrySink struct{}

func (failingTelemetrySink) Insert(context.Context, domain.MeasurementWindow) error {
	return errors.New("simulated tiger data outage")
}

func (failingTelemetrySink) Query(context.Context, domain.TelemetryQuery) ([]domain.TelemetryRecord, error) {
	return nil, errors.New("simulated tiger data outage")
}

func TestTelemetryUnavailableWhenTigerNotConfigured(t *testing.T) {
	// testOwnedApp (ownership_test.go) wires ProjectServices{} -- no
	// Telemetry sink -- exactly like a deployment with TIGER_DATABASE_URL
	// unset. Demo Mode/the simulator/SQLite measurements must still work.
	app := testOwnedApp(t)
	if response := doJSON(t, app, http.MethodGet, "/api/v1/session", nil); response.StatusCode != http.StatusOK {
		t.Fatalf("GET /session with no Tiger configured status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	response := doJSON(t, app, http.MethodGet, "/api/v1/telemetry", nil)
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /telemetry with no Tiger configured status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestTelemetryDualWriteAlongsideSQLiteMeasurements(t *testing.T) {
	app := testProductApp(t)

	// Triggering analysis (as any other session-reading test in this
	// package does) exercises analyzeAt's dual-write: SQLite's
	// SaveMeasurement, then a best-effort Tiger Data insert.
	sessionResponse := doJSON(t, app, http.MethodGet, "/api/v1/session", nil)
	if sessionResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /session status = %d body=%s", sessionResponse.StatusCode, readBody(t, sessionResponse))
	}

	measurementsResponse := doJSON(t, app, http.MethodGet, "/api/v1/measurements?limit=10", nil)
	if measurementsResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /measurements status = %d body=%s", measurementsResponse.StatusCode, readBody(t, measurementsResponse))
	}
	var windows []map[string]any
	decodeBody(t, measurementsResponse, &windows)
	if len(windows) == 0 {
		t.Fatalf("expected SQLite to have at least one measurement window after GET /session")
	}

	telemetryResponse := doJSON(t, app, http.MethodGet, "/api/v1/telemetry?limit=50", nil)
	if telemetryResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /telemetry status = %d body=%s", telemetryResponse.StatusCode, readBody(t, telemetryResponse))
	}
	var records []map[string]any
	decodeBody(t, telemetryResponse, &records)
	if len(records) == 0 {
		t.Fatalf("expected Tiger Data to also have received the dual-written measurement")
	}
}

// TestTelemetryInsertFailureDoesNotBreakTheRequest proves the documented
// dual-write failure semantics: a Tiger Data write error is logged, never
// surfaced to the caller, and never treated as though the measurement
// itself was lost -- SQLite's own SaveMeasurement already succeeded first.
func TestTelemetryInsertFailureDoesNotBreakTheRequest(t *testing.T) {
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
	app := newApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID,
		ProjectServices{Telemetry: failingTelemetrySink{}}, func(ctx *fiber.Ctx) error { return ctx.Next() })

	response := doJSON(t, app, http.MethodGet, "/api/v1/session", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /session with a failing Tiger sink status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	measurementsResponse := doJSON(t, app, http.MethodGet, "/api/v1/measurements?limit=10", nil)
	var windows []map[string]any
	decodeBody(t, measurementsResponse, &windows)
	if len(windows) == 0 {
		t.Fatalf("expected SQLite to still have saved the measurement despite the Tiger Data failure")
	}
}

func TestTelemetryQueryScopesByProbeAndEnforcesLimit(t *testing.T) {
	app := testProductApp(t)
	doJSON(t, app, http.MethodGet, "/api/v1/session", nil)

	response := doJSON(t, app, http.MethodGet, "/api/v1/telemetry?probe=P1&limit=1", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /telemetry?probe=P1 status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var records []map[string]any
	decodeBody(t, response, &records)
	if len(records) > 1 {
		t.Fatalf("len(records) = %d, want at most 1 (limit=1 enforced)", len(records))
	}
	for _, record := range records {
		if record["probe"] != "P1" {
			t.Fatalf("record probe = %v, want P1", record["probe"])
		}
	}
}

func TestTelemetryQueryRejectsInvalidLimit(t *testing.T) {
	app := testProductApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/telemetry?limit=0", nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /telemetry?limit=0 status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/telemetry?limit=999999", nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /telemetry?limit=999999 status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

// TestPatchRemainsLockedRegardlessOfProductData confirms that none of this
// milestone's additions (MongoDB, Tiger Data, users, equipment) loosen the
// hardware-safety gate: POST /patch must still return 423 Locked exactly
// as it did before this milestone, on every app configuration.
func TestPatchRemainsLockedRegardlessOfProductData(t *testing.T) {
	cases := map[string]*fiber.App{
		"without product data":            testOwnedApp(t),
		"with product data and telemetry": testProductApp(t),
	}
	for name, app := range cases {
		t.Run(name, func(t *testing.T) {
			response := doJSON(t, app, http.MethodPost, "/api/v1/patch", nil)
			if response.StatusCode != http.StatusLocked {
				t.Fatalf("POST /patch status = %d, want 423 Locked", response.StatusCode)
			}
		})
	}
}
