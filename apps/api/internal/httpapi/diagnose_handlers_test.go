package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func floatPtr(value float64) *float64 { return &value }

// diagnosisTestProfile mirrors the diagnostics package's own pulseProfile
// test fixture (a single pulse probe "P2"/role "CLOCK"), but with an ID
// matching the given project so it round-trips through the real
// GetProfile(project.ID)/AnalyzeSignals path this handler actually uses.
func diagnosisTestProfile(projectID string) domain.ProjectProfile {
	return domain.ProjectProfile{
		ID:           projectID,
		ProjectName:  "Distance Alarm",
		Controller:   "ESP32",
		LogicVoltage: 3.3,
		Confirmed:    true,
		Probes: []domain.ProbeConfiguration{{
			Probe: "P2",
			Role:  "CLOCK",
			Mode:  domain.ProbeModePulse,
			Expected: domain.ExpectedSignal{
				SignalType: "pulse", Required: true, Stable: true,
				MinFrequencyHz: floatPtr(9), MaxFrequencyHz: floatPtr(11), NominalFrequencyHz: floatPtr(10), MaxDropouts: 0,
			},
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1},
		}},
	}
}

// pulseEnvelopeJSON's sequence must differ between two envelopes submitted
// for the same device in a test -- SaveMeasurement's identity key is
// (source, device_id, sequence, captured_at_ms), and two different-content
// windows submitted under the same identity are correctly rejected as a
// collision (see TestCreateProjectMeasurementIsIdempotentOnRetry for the
// matching-content case, which that same guard treats as a safe retry).
func pulseEnvelopeJSON(sequence uint64, rising uint32, activity []float64) map[string]any {
	periods := []float64{}
	if rising > 0 {
		periods = []float64{100_000, 100_000, 100_000}
	}
	return map[string]any{
		"schema_version": 2,
		"device_id":      "test-device-001",
		"sequence":       sequence,
		"window_ms":      1000,
		"samples": []map[string]any{{
			"probe": "P2", "mode": "pulse", "edge_count": rising * 2, "rising_edges": rising, "falling_edges": rising,
			"periods_us": periods, "high_pulse_widths_us": []float64{50_000, 50_000, 50_000}, "activity_counts": activity,
		}},
	}
}

func TestCreateProjectMeasurementRequiresAnExistingProfile(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d body=%s, want 409 PROFILE_NOT_FOUND", response.StatusCode, readBody(t, response))
	}
}

func TestCreateProjectMeasurementAnalyzesAndPersistsDeterministically(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	if err := repository.SaveProfile(diagnosisTestProfile(project.ID)); err != nil {
		t.Fatal(err)
	}

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var window domain.MeasurementWindow
	decodeBody(t, response, &window)
	if window.ProfileID != project.ID || window.Source != "serial" {
		t.Fatalf("window = %#v", window)
	}
	facts, ok := window.Analysis.Probe("P2")
	if !ok || facts.FrequencyHz == nil || *facts.FrequencyHz < 9 || *facts.FrequencyHz > 11 {
		t.Fatalf("analysis facts = %#v, want a resolved ~10Hz frequency", facts)
	}
}

func TestCreateProjectMeasurementIsIdempotentOnRetry(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	if err := repository.SaveProfile(diagnosisTestProfile(project.ID)); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})}

	first := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", body)
	var firstWindow domain.MeasurementWindow
	decodeBody(t, first, &firstWindow)

	second := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", body)
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("retry status = %d body=%s, want the retry to succeed idempotently", second.StatusCode, readBody(t, second))
	}
	var secondWindow domain.MeasurementWindow
	decodeBody(t, second, &secondWindow)
	if secondWindow.ID != firstWindow.ID {
		t.Fatalf("retrying the exact same measurement produced a new row: first=%d second=%d, want the same id", firstWindow.ID, secondWindow.ID)
	}
}

func TestCreateProjectMeasurementRejectsProfileIDMismatch(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	if err := repository.SaveProfile(diagnosisTestProfile(project.ID)); err != nil {
		t.Fatal(err)
	}
	envelope := pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})
	envelope["profile_id"] = "some-other-project"

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": envelope})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s, want 422 PROFILE_ID_MISMATCH", response.StatusCode, readBody(t, response))
	}
}

func TestCreateProjectMeasurementRejectsInvalidSource(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	if err := repository.SaveProfile(diagnosisTestProfile(project.ID)); err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"source": "gemini-imagined", "envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s, want 422 INVALID_MEASUREMENT_SOURCE", response.StatusCode, readBody(t, response))
	}
}

func TestGetProjectDiagnosisReportsInsufficientEvidenceWithNoMeasurement(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	response := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/diagnose", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var diagnosis domain.ProjectDiagnosis
	decodeBody(t, response, &diagnosis)
	if diagnosis.Status != domain.ProjectDiagnosisInsufficientEvidence {
		t.Fatalf("status = %s, want INSUFFICIENT_EVIDENCE", diagnosis.Status)
	}
	if diagnosis.Diagnosis.Confidence != 0 || diagnosis.MeasurementID != nil {
		t.Fatalf("diagnosis = %#v, must not fabricate confidence or a measurement id", diagnosis.Diagnosis)
	}
}

func TestGetProjectDiagnosisRunsRealDeterministicDiagnosisWithAMeasurement(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	if err := repository.SaveProfile(diagnosisTestProfile(project.ID)); err != nil {
		t.Fatal(err)
	}
	doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": pulseEnvelopeJSON(1, 0, []float64{0, 0, 0, 0})})

	response := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/diagnose", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var diagnosis domain.ProjectDiagnosis
	decodeBody(t, response, &diagnosis)
	if diagnosis.Status != domain.ProjectDiagnosisComplete || diagnosis.MeasurementID == nil {
		t.Fatalf("diagnosis = %#v", diagnosis)
	}
	if !strings.Contains(diagnosis.Diagnosis.Headline, "Missing") {
		t.Fatalf("headline = %q, want it to reflect the missing-signal rule that actually fired", diagnosis.Diagnosis.Headline)
	}
}

// TestGetProjectDiagnosisFullPhysicalGitScenario is this milestone's
// deterministic end-to-end scenario (Physical Git + measurements -> bounded
// evidence -> PROBE): HW-001 (working) and HW-002 (changed, broken) with two
// measurements, one before and one after. It asserts PROBE receives factual
// Physical Git context and that nothing in the response asserts causation
// that the evidence does not establish.
func TestGetProjectDiagnosisFullPhysicalGitScenario(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)

	workingProfile := diagnosisTestProfile(project.ID)
	workingProfile.Components = []domain.ComponentSpecification{{ID: "hc-sr04-1", Name: "HC-SR04"}}
	if err := repository.SaveProfile(workingProfile); err != nil {
		t.Fatal(err)
	}
	hw1Response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", map[string]any{"note": "working baseline"})
	var hw1 domain.PhysicalCommit
	decodeBody(t, hw1Response, &hw1)

	measurement1Response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})})
	if measurement1Response.StatusCode != http.StatusCreated {
		t.Fatalf("measurement 1 status = %d body=%s", measurement1Response.StatusCode, readBody(t, measurement1Response))
	}

	brokenProfile := diagnosisTestProfile(project.ID)
	brokenProfile.Components = []domain.ComponentSpecification{{ID: "hc-sr04-1", Name: "HC-SR04"}, {ID: "sg90-1", Name: "SG90 Servo"}}
	if err := repository.SaveProfile(brokenProfile); err != nil {
		t.Fatal(err)
	}
	hw2Response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", map[string]any{"note": "added servo, now broken"})
	var hw2 domain.PhysicalCommit
	decodeBody(t, hw2Response, &hw2)
	if hw1.ID == hw2.ID {
		t.Fatal("expected two distinct physical commits")
	}

	measurement2Response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", map[string]any{"envelope": pulseEnvelopeJSON(2, 0, []float64{0, 0, 0, 0})})
	if measurement2Response.StatusCode != http.StatusCreated {
		t.Fatalf("measurement 2 status = %d body=%s", measurement2Response.StatusCode, readBody(t, measurement2Response))
	}
	var measurement2 domain.MeasurementWindow
	decodeBody(t, measurement2Response, &measurement2)

	response := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/diagnose", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("diagnose status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var diagnosis domain.ProjectDiagnosis
	decodeBody(t, response, &diagnosis)

	if diagnosis.Status != domain.ProjectDiagnosisComplete {
		t.Fatalf("status = %s, want DIAGNOSED", diagnosis.Status)
	}
	if diagnosis.MeasurementID == nil || *diagnosis.MeasurementID != measurement2.ID {
		t.Fatalf("measurement_id = %v, want the latest measurement %d", diagnosis.MeasurementID, measurement2.ID)
	}
	if diagnosis.ReferenceMeasurementID == nil {
		t.Fatal("expected a reference measurement id now that two measurements exist")
	}
	if len(diagnosis.Evidence.PhysicalContext) == 0 {
		t.Fatal("expected Physical Git context describing the change between HW-001 and HW-002")
	}
	foundComponentChange := false
	for _, fact := range diagnosis.Evidence.PhysicalContext {
		if fact.Provenance != domain.ProvenancePhysicalHistory {
			t.Fatalf("physical context fact = %#v, want PHYSICAL_HISTORY provenance", fact)
		}
		text, _ := fact.Value.(string)
		if strings.Contains(text, "component") {
			foundComponentChange = true
		}
		for _, causal := range []string{"caused", "because", "led to", "due to"} {
			if strings.Contains(strings.ToLower(text), causal) {
				t.Fatalf("physical context fact %q asserts causation, which Physical Git context must never do", text)
			}
		}
	}
	if !foundComponentChange {
		t.Fatalf("physical context = %#v, want a fact describing the added SG90 component", diagnosis.Evidence.PhysicalContext)
	}
	for _, causal := range []string{"caused", "because it", "led to"} {
		if strings.Contains(strings.ToLower(diagnosis.Diagnosis.Summary), causal) {
			t.Fatalf("diagnosis summary %q asserts causation the deterministic rules never established", diagnosis.Diagnosis.Summary)
		}
	}
}

func TestProjectMeasurementAndDiagnosisOwnershipIsolation(t *testing.T) {
	app := testOwnedApp(t)
	createResponse := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Alice's Rig", "controller": "ESP32", "logic_voltage": 3.3})
	var project domain.Project
	decodeBody(t, createResponse, &project)

	measurementResponse := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/measurements", "owner-b", map[string]any{"envelope": pulseEnvelopeJSON(1, 10, []float64{1, 1, 1, 1})})
	if measurementResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("a different owner must not be able to submit a measurement to another owner's project: status = %d body=%s", measurementResponse.StatusCode, readBody(t, measurementResponse))
	}
	diagnoseResponse := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/diagnose", "owner-b", nil)
	if diagnoseResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("a different owner must not be able to read another owner's diagnosis: status = %d body=%s", diagnoseResponse.StatusCode, readBody(t, diagnoseResponse))
	}
}

// TestGetProjectDiagnosisWorksWithAStoredMeasurementLackingASchemaVersion
// is a direct regression test for a real bug found via manual testing
// against the seeded physical-git-demo project: its measurements are
// persisted directly (bypassing the HTTP envelope path this handler's
// sibling createProjectMeasurement validates), so Raw.SchemaVersion is the
// zero value. Diagnosis must not depend on being able to re-run signal
// analysis on that raw envelope -- it must work from the measurement's own
// already-persisted, already-valid Analysis.
func TestGetProjectDiagnosisWorksWithAStoredMeasurementLackingASchemaVersion(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	profile := diagnosisTestProfile(project.ID)
	if err := repository.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveMeasurement(domain.MeasurementWindow{
		ProfileID: project.ID, Source: "serial", DeviceID: "legacy-device", Sequence: 1, CapturedAtMS: 1,
		Raw:      domain.TelemetryEnvelope{DeviceID: "legacy-device", ProfileID: project.ID},
		Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{{Probe: "P2", Role: "CLOCK", Stable: true}}},
	}); err != nil {
		t.Fatal(err)
	}

	response := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/diagnose", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200 (must not depend on re-analyzing an incompatible/missing raw envelope)", response.StatusCode, readBody(t, response))
	}
	var diagnosis domain.ProjectDiagnosis
	decodeBody(t, response, &diagnosis)
	if diagnosis.Status != domain.ProjectDiagnosisComplete {
		t.Fatalf("diagnosis = %#v", diagnosis)
	}
}

func TestGetProjectDiagnosisMalformedProjectID(t *testing.T) {
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/projects/does-not-exist-at-all/diagnose", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}
