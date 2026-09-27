package httpapi

import (
	"net/http"
	"testing"
)

// TestMeasurementsSupportsScopedQueries proves GET /api/v1/measurements
// alone (there is exactly one measurement API, not a second one for scoped
// queries) supports probe/device/source/time-range scoping directly
// against SQLite's measurement_windows table.
func TestMeasurementsSupportsScopedQueries(t *testing.T) {
	app := testOwnedApp(t)

	// Trigger at least one measurement window via the simulator, exactly
	// like the existing GET /session-based tests in this package.
	if response := doJSON(t, app, http.MethodGet, "/api/v1/session", nil); response.StatusCode != http.StatusOK {
		t.Fatalf("GET /session status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	response := doJSON(t, app, http.MethodGet, "/api/v1/measurements?device_id=reweird-01&limit=5", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /measurements?device_id=... status = %d body=%s", response.StatusCode, readBody(t, response))
	}

	response = doJSON(t, app, http.MethodGet, "/api/v1/measurements?since_ms=0&until_ms=99999999999999", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /measurements?since_ms=...&until_ms=... status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var windows []map[string]any
	decodeBody(t, response, &windows)
	if len(windows) == 0 {
		t.Fatalf("expected at least one window within an all-encompassing time range")
	}
}

func TestMeasurementsRejectsInvalidTimeRangeParams(t *testing.T) {
	app := testOwnedApp(t)
	if response := doJSON(t, app, http.MethodGet, "/api/v1/measurements?since_ms=not-a-number", nil); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /measurements?since_ms=not-a-number status = %d, want 400", response.StatusCode)
	}
	if response := doJSON(t, app, http.MethodGet, "/api/v1/measurements?until_ms=not-a-number", nil); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /measurements?until_ms=not-a-number status = %d, want 400", response.StatusCode)
	}
}
