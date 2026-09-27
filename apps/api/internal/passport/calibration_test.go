package passport

import (
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
)

func physicalRun(t *testing.T, profile domain.ProjectProfile, count int, source string, mutate func(index int, envelope *domain.TelemetryEnvelope)) []domain.MeasurementWindow {
	t.Helper()
	analyzer := signalanalysis.New()
	windows := make([]domain.MeasurementWindow, 0, count)
	for index := 0; index < count; index++ {
		envelope := physicalfixture.Frame(uint64(100 + index))
		if mutate != nil {
			mutate(index, &envelope)
		}
		analysis, err := analyzer.Analyze(envelope, profile)
		if err != nil {
			t.Fatal(err)
		}
		windows = append(windows, domain.MeasurementWindow{
			ID: int64(500 + index), ProfileID: profile.ID, Source: source, DeviceID: envelope.DeviceID,
			Sequence: envelope.Sequence, CapturedAtMS: 0, IngestedAtMS: int64(10_000 + index*1000), Raw: envelope, Analysis: analysis,
		})
	}
	return windows
}

func calibrationInput(profile domain.ProjectProfile, windows []domain.MeasurementWindow) CalibrationInput {
	return CalibrationInput{Profile: profile, ActiveSource: "serial", Windows: windows, PhysicalAllowed: true}
}

func TestSimulatedCapturesCannotEstablishPhysicalKnownGood(t *testing.T) {
	profile := physicalfixture.Profile()
	simulated := physicalRun(t, profile, CalibrationWindows, "simulator", nil)
	if _, err := LearnKnownGood(profile, simulated, ""); err == nil || !strings.Contains(err.Error(), "never establish a physical Known Good") {
		t.Fatalf("simulated windows produced a physical baseline: %v", err)
	}
	mixed := physicalRun(t, profile, CalibrationWindows, "serial", nil)
	mixed[4].Source = "simulator"
	if _, err := LearnKnownGood(profile, mixed, ""); err == nil {
		t.Fatal("a run containing a simulated window was accepted")
	}
	state := ComputeCalibration(CalibrationInput{Profile: profile, ActiveSource: "simulator", Windows: simulated, PhysicalAllowed: true})
	if state.Status != domain.CalibrationSimulatedSource || state.CanSaveKnownGood || state.WindowsObserved != 0 {
		t.Fatalf("simulated source offered physical calibration: %+v", state)
	}
}

func TestPhysicalBaselineRetainsRealSerialProvenanceAndBinding(t *testing.T) {
	profile := physicalfixture.Profile()
	run := physicalRun(t, profile, CalibrationWindows, "serial", nil)
	baseline, err := LearnKnownGood(profile, run, " bench verified ")
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Source != domain.BaselinePhysical || baseline.Provenance != domain.ProvenanceRealSerial {
		t.Fatalf("baseline provenance = %s/%s", baseline.Source, baseline.Provenance)
	}
	if baseline.DeviceID != physicalfixture.DeviceID || baseline.ProfileID != profile.ID || baseline.ProfileVersion != profile.Version {
		t.Fatalf("baseline not tied to device/profile revision: %+v", baseline)
	}
	if baseline.ProbeMappingHash != MappingHash(profile) || len(baseline.ProbeMapping) != 6 || baseline.ProbeMapping[3].Role != "SERVO" {
		t.Fatalf("baseline lost its probe mapping: %+v", baseline.ProbeMapping)
	}
	if baseline.WindowCount != CalibrationWindows || baseline.FirstMeasurementID != run[0].ID || baseline.LastMeasurementID != run[len(run)-1].ID || baseline.MeasurementID != run[len(run)-1].ID {
		t.Fatalf("baseline does not reference its observed windows: %+v", baseline)
	}
	if baseline.SavedAtMS <= 0 || baseline.Note != "bench verified" {
		t.Fatalf("baseline timestamp/note = %d %q", baseline.SavedAtMS, baseline.Note)
	}
	observedOptional := false
	for _, probe := range baseline.Probes {
		if probe.Trusted.Status != domain.BaselineUserConfirmedHealthy || probe.Trusted.WindowCount != CalibrationWindows {
			t.Fatalf("%s trusted = %+v", probe.Probe, probe.Trusted)
		}
		if probe.Probe == "P5" {
			observedOptional = true
		}
		if probe.Probe == "P6" {
			t.Fatal("unassigned P6 entered the Known Good")
		}
	}
	if !observedOptional {
		t.Fatal("observed analog P5 was omitted from Known Good")
	}
}

func TestBaselineIsTiedToDeviceProfileRevisionAndMapping(t *testing.T) {
	profile := physicalfixture.Profile()
	run := physicalRun(t, profile, CalibrationWindows, "serial", nil)
	baseline, err := LearnKnownGood(profile, run, "")
	if err != nil {
		t.Fatal(err)
	}
	later := physicalRun(t, profile, CalibrationWindows+1, "serial", nil)[CalibrationWindows]
	if status, detail := CurrentStatus(profile, &later, &baseline, true); status != domain.PassportHealthy {
		t.Fatalf("matching later capture = %s: %s", status, detail)
	}
	otherDevice := later
	otherDevice.DeviceID, otherDevice.Raw.DeviceID, otherDevice.Analysis.DeviceID = "reweird-OTHER", "reweird-OTHER", "reweird-OTHER"
	if status, _ := CurrentStatus(profile, &otherDevice, &baseline, true); status == domain.PassportHealthy {
		t.Fatal("another device was verified against this device's baseline")
	}

	revised := profile
	revised.Version++
	if status, _ := CurrentStatus(revised, &later, &baseline, true); status != domain.PassportBaselineIncompatible {
		t.Fatalf("new profile revision accepted old baseline: %s", status)
	}
	if applied := ApplyKnownGood(revised, domain.BaselinePhysical, &baseline); applied.Probes[0].Baseline != nil {
		t.Fatal("old-revision baseline applied to the new revision")
	}
	remapped := physicalfixture.Profile()
	remapped.Probes = append([]domain.ProbeConfiguration(nil), profile.Probes...)
	remapped.Probes[4].SafeMeasurement.InputScale = 2 // e.g. a divider added to ZMPT OUT
	if ok, reason := BaselineCompatible(remapped, &baseline); ok || !strings.Contains(reason, "probe mapping") {
		t.Fatalf("remapped probes accepted old baseline: %v %s", ok, reason)
	}
	state := ComputeCalibration(CalibrationInput{Profile: revised, ActiveSource: "serial", Windows: run, KnownGood: &baseline, PhysicalAllowed: true})
	if state.Status != domain.CalibrationBaselineIncompatible || state.CanSaveKnownGood {
		t.Fatalf("revision change was not marked incompatible: %+v", state)
	}
}

func TestLaterPhysicalCaptureIsComparedWithKnownGood(t *testing.T) {
	profile := physicalfixture.Profile()
	baseline, err := LearnKnownGood(profile, physicalRun(t, profile, CalibrationWindows, "serial", nil), "")
	if err != nil {
		t.Fatal(err)
	}
	calibrated := ApplyKnownGood(profile, domain.BaselinePhysical, &baseline)
	// The rail drops to 4.88 V: inside the configured 4.75–5.25 V limits but
	// outside what this circuit did when the user confirmed it healthy.
	sagging := physicalfixture.Frame(200)
	sagging.Samples[0].AnalogMV = []float64{2440, 2441, 2439, 2440, 2442, 2440, 2438, 2440}
	analysis, err := signalanalysis.New().Analyze(sagging, calibrated)
	if err != nil {
		t.Fatal(err)
	}
	rail, _ := analysis.Probe("P1")
	if !rail.Stable || len(rail.KnownGoodDeviations) == 0 {
		t.Fatalf("in-spec but abnormal rail was not compared with Known Good: %+v", rail)
	}
	window := domain.MeasurementWindow{ID: 900, ProfileID: profile.ID, Source: "serial", DeviceID: physicalfixture.DeviceID, Sequence: 200, IngestedAtMS: 99_000, Raw: sagging, Analysis: analysis}
	if status, detail := CurrentStatus(profile, &window, &baseline, true); status != domain.PassportDeviation || !strings.Contains(detail, "Known Good") {
		t.Fatalf("deviation not reported against Known Good: %s %s", status, detail)
	}
	state := ComputeCalibration(CalibrationInput{Profile: profile, ActiveSource: "serial", Windows: []domain.MeasurementWindow{window}, KnownGood: &baseline, PhysicalAllowed: true})
	if state.Status != domain.CalibrationCalibrated || state.Probes[0].KnownGood == nil || len(state.Probes[0].Issues) == 0 {
		t.Fatalf("calibrated state does not show EXPECTED/OBSERVED/KNOWN GOOD with the deviation: %+v", state.Probes[0])
	}
}

func TestCalibrationCannotDeclareUnhealthyHardwareHealthy(t *testing.T) {
	profile := physicalfixture.Profile()
	observing := ComputeCalibration(calibrationInput(profile, physicalRun(t, profile, 4, "serial", nil)))
	if observing.Status != domain.CalibrationObserving || observing.CanSaveKnownGood || observing.WindowsObserved != 4 {
		t.Fatalf("partial observation offered save: %+v", observing)
	}
	ready := ComputeCalibration(calibrationInput(profile, physicalRun(t, profile, CalibrationWindows, "serial", nil)))
	if ready.Status != domain.CalibrationReviewRequired || !ready.CanSaveKnownGood {
		t.Fatalf("stable observation not offered for review: %+v", ready)
	}
	for name, mutate := range map[string]func(int, *domain.TelemetryEnvelope){
		"unreliable trigger capture": func(index int, envelope *domain.TelemetryEnvelope) {
			if index == 7 {
				envelope.Samples[1] = physicalfixture.LegacyTrigSample()
			}
		},
		"rail below specification": func(index int, envelope *domain.TelemetryEnvelope) {
			if index == 2 {
				envelope.Samples[0].AnalogMV = []float64{2200, 2201, 2199}
			}
		},
		"missing echo": func(index int, envelope *domain.TelemetryEnvelope) {
			if index == 9 {
				zero := 0
				envelope.Samples[2] = domain.TelemetrySample{Probe: "P3", Mode: domain.ProbeModePulse, State: &zero, MaxGapUS: 1_000_000, ActivityCounts: make([]float64, 10)}
			}
		},
	} {
		run := physicalRun(t, profile, CalibrationWindows, "serial", mutate)
		state := ComputeCalibration(calibrationInput(profile, run))
		if state.Status != domain.CalibrationAttentionRequired || state.CanSaveKnownGood || len(state.Blockers) == 0 {
			t.Fatalf("%s: unhealthy observation offered as Known Good: %+v", name, state)
		}
		if _, err := LearnKnownGood(profile, run, ""); err == nil {
			t.Fatalf("%s: unhealthy run learned as Known Good", name)
		}
	}
	interrupted := physicalRun(t, profile, CalibrationWindows, "serial", nil)
	interrupted[5].Sequence += 7
	if _, err := LearnKnownGood(profile, interrupted, ""); err == nil {
		t.Fatal("interrupted observation was learned")
	}
	demoOnly := ComputeCalibration(CalibrationInput{Profile: profile, ActiveSource: "serial", Windows: physicalRun(t, profile, CalibrationWindows, "serial", nil), PhysicalAllowed: false, PhysicalBlock: "demo profile"})
	if demoOnly.CanSaveKnownGood || demoOnly.Status != domain.CalibrationNotCalibrated {
		t.Fatalf("profile without confirmed probes offered calibration: %+v", demoOnly)
	}
}

func TestUnassignedActivityIsReportedNotIgnored(t *testing.T) {
	profile := physicalfixture.Profile()
	run := physicalRun(t, profile, CalibrationWindows, "serial", func(_ int, envelope *domain.TelemetryEnvelope) {
		zero := 0
		envelope.Samples[5] = domain.TelemetrySample{Probe: "P6", Mode: domain.ProbeModeDigital, State: &zero, EdgeCount: 100, RisingEdges: 50, FallingEdges: 50, MaxGapUS: 19_000, ActivityCounts: make([]float64, 10)}
	})
	state := ComputeCalibration(calibrationInput(profile, run))
	p6 := state.Probes[5]
	if p6.Probe != "P6" || p6.Observed.ActiveWindows != CalibrationWindows || len(p6.Issues) == 0 {
		t.Fatalf("activity on unassigned P6 was hidden: %+v", p6)
	}
	if state.Status != domain.CalibrationReviewRequired {
		t.Fatalf("an unassigned probe blocked calibration: %+v", state.Blockers)
	}
}
