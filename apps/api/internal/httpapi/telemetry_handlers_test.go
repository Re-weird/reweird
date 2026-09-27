package httpapi

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

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
