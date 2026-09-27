package httpapi

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

type fixedSerialSource struct{ envelope domain.TelemetryEnvelope }

func (source fixedSerialSource) Name() string { return "serial" }
func (source fixedSerialSource) Latest(context.Context) (domain.TelemetryEnvelope, error) {
	return source.envelope, nil
}

func TestPhysicalCalibrationWorkflowOverHTTP(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "calibration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	profile := physicalfixture.Profile()
	project := domain.Project{ID: profile.ID, Name: profile.ProjectName, Controller: "ESP32", LogicVoltage: 3.3, AnalysisStatus: domain.AnalysisConfirmed,
		ProbePlan: &domain.ProbePlan{ProjectID: profile.ID, ProfileID: profile.ID, Connected: true, ConnectedAtMS: 1}}
	if err := repository.SaveProjectProfile(project, profile); err != nil {
		t.Fatal(err)
	}
	app := NewApp(diagnostics.NewEngine(signalanalysis.New()), repository, fixedSerialSource{physicalfixture.Frame(1)}, profile.ID, ProjectServices{UploadRoot: t.TempDir()}, false)
	path := "/api/v1/profiles/" + profile.ID

	var state domain.CalibrationState
	decodeBody(t, doJSON(t, app, http.MethodGet, path+"/calibration", nil), &state)
	if state.Status != domain.CalibrationNotCalibrated || state.CanSaveKnownGood {
		t.Fatalf("empty calibration = %+v", state)
	}

	analyzer := signalanalysis.New()
	var last domain.MeasurementWindow
	for index := 0; index < passport.CalibrationWindows; index++ {
		envelope := physicalfixture.Frame(uint64(1 + index))
		analysis, err := analyzer.Analyze(envelope, profile)
		if err != nil {
			t.Fatal(err)
		}
		last, err = repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: profile.ID, Source: "serial", DeviceID: envelope.DeviceID, Sequence: envelope.Sequence, IngestedAtMS: int64(10 + index), Raw: envelope, Analysis: analysis})
		if err != nil {
			t.Fatal(err)
		}
	}
	decodeBody(t, doJSON(t, app, http.MethodGet, path+"/calibration", nil), &state)
	if state.Status != domain.CalibrationReviewRequired || !state.CanSaveKnownGood || state.Provenance != domain.ProvenanceRealSerial || state.CandidateMeasurementID != last.ID {
		t.Fatalf("observed calibration = %+v", state)
	}
	if state.Probes[1].Expected.MinPulseWidthUS == nil || state.Probes[1].Observed.AveragePulseWidthUS == nil || state.Probes[1].KnownGood != nil {
		t.Fatalf("EXPECTED / OBSERVED / KNOWN GOOD were not kept separate: %+v", state.Probes[1])
	}

	unconfirmed := doJSON(t, app, http.MethodPost, path+"/known-good", map[string]any{"measurement_id": last.ID})
	if unconfirmed.StatusCode != http.StatusBadRequest {
		t.Fatalf("Known Good saved without explicit healthy confirmation: %d", unconfirmed.StatusCode)
	}
	saved := doJSON(t, app, http.MethodPost, path+"/known-good", map[string]any{"measurement_id": last.ID, "confirm_healthy": true, "source": "SIMULATED", "provenance": "SIMULATED"})
	if saved.StatusCode != http.StatusCreated {
		t.Fatalf("save status %d: %s", saved.StatusCode, readBody(t, saved))
	}
	var baseline domain.KnownGoodBaseline
	decodeBody(t, saved, &baseline)
	if baseline.Source != domain.BaselinePhysical || baseline.Provenance != domain.ProvenanceRealSerial || baseline.WindowCount != passport.CalibrationWindows {
		t.Fatalf("client influenced provenance: %+v", baseline)
	}
	decodeBody(t, doJSON(t, app, http.MethodGet, path+"/calibration", nil), &state)
	if state.Status != domain.CalibrationCalibrated || state.KnownGood == nil || state.Probes[0].KnownGood == nil {
		t.Fatalf("calibrated state = %+v", state)
	}

	revised := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+profile.ID+"/profile/revise", nil)
	if revised.StatusCode != http.StatusOK {
		t.Fatalf("revise status %d: %s", revised.StatusCode, readBody(t, revised))
	}
	var revision domain.ProjectProfile
	decodeBody(t, revised, &revision)
	if revision.Version != profile.Version+1 || revision.Confirmed || len(revision.Probes) != 0 {
		t.Fatalf("revision = %+v", revision)
	}
	if blocked := doJSON(t, app, http.MethodGet, path+"/calibration", nil); blocked.StatusCode != http.StatusConflict {
		t.Fatalf("draft revision calibration status = %d", blocked.StatusCode)
	}
	revision.Confirmed = true
	revision.Probes = profile.Probes
	project.ProbePlan = &domain.ProbePlan{ProjectID: profile.ID, ProfileID: profile.ID, Connected: true, ConnectedAtMS: 1}
	if err := repository.SaveProjectProfile(project, revision); err != nil {
		t.Fatal(err)
	}
	decodeBody(t, doJSON(t, app, http.MethodGet, path+"/calibration", nil), &state)
	if state.Status != domain.CalibrationBaselineIncompatible || state.CanSaveKnownGood || state.WindowsObserved != 0 {
		t.Fatalf("old-revision baseline was not marked incompatible: %+v", state)
	}
}

func TestSimulatorCannotCalibratePhysicalHardware(t *testing.T) {
	app, _ := testApp(t)
	doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": "healthy"})
	var state domain.CalibrationState
	decodeBody(t, doJSON(t, app, http.MethodGet, "/api/v1/profiles/ultrasonic-demo/calibration", nil), &state)
	if state.Status != domain.CalibrationSimulatedSource || state.CanSaveKnownGood {
		t.Fatalf("simulator calibration = %+v", state)
	}
}
