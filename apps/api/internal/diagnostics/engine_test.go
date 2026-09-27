package diagnostics

import (
	"context"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
)

func TestGenericRules(t *testing.T) {
	tests := []struct {
		name       string
		profile    domain.ProjectProfile
		envelope   domain.TelemetryEnvelope
		stage      domain.Stage
		wantRule   string
		wantStatus string
	}{
		{
			name:       "healthy signal",
			profile:    pulseProfile(trustedFrequencyBaseline(10)),
			envelope:   pulseEnvelope(10, []float64{1, 1, 1, 1}),
			stage:      domain.StageDiagnose,
			wantRule:   "expected-activity",
			wantStatus: "pass",
		},
		{
			name:       "dead signal",
			profile:    pulseProfile(trustedFrequencyBaseline(10)),
			envelope:   pulseEnvelope(0, []float64{0, 0, 0, 0}),
			stage:      domain.StageDiagnose,
			wantRule:   "missing-signal",
			wantStatus: "fail",
		},
		{
			name:       "low voltage",
			profile:    powerProfile([]float64{4.75, 5.25}),
			envelope:   analogEnvelope([]float64{2000, 2020, 2010}),
			stage:      domain.StageDiagnose,
			wantRule:   "voltage-outside-specification",
			wantStatus: "fail",
		},
		{
			name:       "intermittent signal",
			profile:    pulseProfile(trustedFrequencyBaseline(10)),
			envelope:   pulseEnvelope(7, []float64{1, 0, 1, 0}),
			stage:      domain.StageDiagnose,
			wantRule:   "unexpected-dropout",
			wantStatus: "fail",
		},
		{
			name:       "unstable power rail",
			profile:    powerProfile([]float64{4.75, 5.25}),
			envelope:   analogEnvelope([]float64{2600, 2400, 2600, 2400}),
			stage:      domain.StageDiagnose,
			wantRule:   "power-rail-instability",
			wantStatus: "fail",
		},
	}

	engine := NewEngine(signalanalysis.New())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.envelope.ProfileID = test.profile.ID
			session, err := engine.AnalyzeEnvelope(context.Background(), test.profile, test.stage, "test", test.envelope, nil)
			if err != nil {
				t.Fatalf("AnalyzeEnvelope() error = %v", err)
			}
			assertRule(t, session.Evidence.RuleResults, test.wantRule, test.wantStatus)
		})
	}
}

func TestSimultaneousDropoutSuggestsSharedCause(t *testing.T) {
	profile := pulseProfile(trustedFrequencyBaseline(10))
	second := profile.Probes[0]
	second.Probe = "P3"
	second.Role = "SECOND_SIGNAL"
	profile.Probes = append(profile.Probes, second)
	envelope := pulseEnvelope(9, []float64{1, 0, 1})
	envelope.Samples = append(envelope.Samples, pulseSample("P3", 9, []float64{1, 0, 1}))

	session, err := NewEngine(signalanalysis.New()).AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "test", envelope, nil)
	if err != nil {
		t.Fatalf("AnalyzeEnvelope() error = %v", err)
	}
	assertRule(t, session.Evidence.RuleResults, "simultaneous-dropout", "fail")
	if !strings.Contains(strings.ToLower(session.Diagnosis.Summary), "shared") {
		t.Fatalf("diagnosis summary %q does not identify a shared cause", session.Diagnosis.Summary)
	}
}

func TestMovementCorrelatedDropout(t *testing.T) {
	profile := pulseProfile(trustedFrequencyBaseline(10))
	engine := NewEngine(signalanalysis.New())
	before, err := signalanalysis.New().Analyze(pulseEnvelope(9, []float64{1, 1, 1, 0}), profile)
	if err != nil {
		t.Fatalf("analyze reference: %v", err)
	}

	session, err := engine.AnalyzeEnvelope(context.Background(), profile, domain.StageTest, "test", pulseEnvelope(5, []float64{0, 1, 0, 0}), &before)
	if err != nil {
		t.Fatalf("AnalyzeEnvelope() error = %v", err)
	}
	assertRule(t, session.Evidence.RuleResults, "movement-correlation", "fail")
	if !strings.Contains(session.Diagnosis.Summary, "strongly supports") {
		t.Fatalf("diagnosis summary %q does not use evidence-qualified language", session.Diagnosis.Summary)
	}
}

func TestHealthyBaselineComparison(t *testing.T) {
	profile := pulseProfile(trustedFrequencyBaseline(10))
	analysis, err := signalanalysis.New().Analyze(pulseEnvelope(10, []float64{1, 1, 1}), profile)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	facts, _ := analysis.Probe("P2")
	if facts.BaselineDeviationPercent == nil || *facts.BaselineDeviationPercent > 0.01 {
		t.Fatalf("baseline deviation = %v, want approximately 0", facts.BaselineDeviationPercent)
	}
}

func TestMissingBaselineDoesNotBecomeEvidence(t *testing.T) {
	profile := pulseProfile(nil)
	session, err := NewEngine(signalanalysis.New()).AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "test", pulseEnvelope(7, []float64{1, 0, 1}), nil)
	if err != nil {
		t.Fatalf("AnalyzeEnvelope() error = %v", err)
	}
	if len(session.Evidence.BaselineComparison) != 0 {
		t.Fatalf("BaselineComparison = %#v, want empty without a trusted baseline", session.Evidence.BaselineComparison)
	}
	if len(session.Evidence.UnresolvedQuestions) == 0 || !strings.Contains(session.Evidence.UnresolvedQuestions[0], "No trusted healthy baseline") {
		t.Fatalf("UnresolvedQuestions = %#v, want missing-baseline disclosure", session.Evidence.UnresolvedQuestions)
	}
}

// fakeProviderCapture is a probe.Provider test double that records exactly
// what Evidence it was called with, so a test can assert what did and
// did not reach PROBE without needing a real Gemini call.
type fakeProviderCapture struct {
	received domain.Evidence
	called   int
}

func (fake *fakeProviderCapture) Diagnose(_ context.Context, evidence domain.Evidence, deterministic domain.Diagnosis) domain.Diagnosis {
	fake.received = evidence
	fake.called++
	return deterministic
}

func TestAnalyzeEnvelopeWithContextMergesPhysicalContextIntoEvidenceProbeSees(t *testing.T) {
	fake := &fakeProviderCapture{}
	engine := NewEngineWithProbe(signalanalysis.New(), fake)
	profile := pulseProfile(nil)
	physicalContext := []domain.EvidenceFact{
		{Name: "physical_history_change", Value: "HW-001 -> HW-002: ECHO connection changed.", Provenance: domain.ProvenancePhysicalHistory},
		{Name: "vision_interpreted_component", Value: "SG90 Servo", Provenance: domain.ProvenanceAIInterpretation},
	}

	session, err := engine.AnalyzeEnvelopeWithContext(context.Background(), profile, domain.StageDiagnose, "test", pulseEnvelope(7, []float64{1, 0, 1}), nil, physicalContext)
	if err != nil {
		t.Fatalf("AnalyzeEnvelopeWithContext() error = %v", err)
	}
	if fake.called != 1 {
		t.Fatalf("probe called %d times, want exactly 1", fake.called)
	}
	if len(fake.received.PhysicalContext) != 2 {
		t.Fatalf("PROBE received PhysicalContext = %#v, want the 2 facts passed in", fake.received.PhysicalContext)
	}
	if len(session.Evidence.PhysicalContext) != 2 {
		t.Fatalf("session.Evidence.PhysicalContext = %#v, want the 2 facts to also be on the returned Evidence", session.Evidence.PhysicalContext)
	}
	// Every measured/derived fact from the ordinary deterministic path must
	// still be present -- merging physical context must never replace it.
	if len(session.Evidence.DerivedFacts) == 0 {
		t.Fatal("DerivedFacts were lost when PhysicalContext was merged in")
	}
}

func TestAnalyzeEnvelopeWithoutContextLeavesPhysicalContextEmpty(t *testing.T) {
	engine := NewEngine(signalanalysis.New())
	profile := pulseProfile(nil)
	session, err := engine.AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "test", pulseEnvelope(7, []float64{1, 0, 1}), nil)
	if err != nil {
		t.Fatalf("AnalyzeEnvelope() error = %v", err)
	}
	if len(session.Evidence.PhysicalContext) != 0 {
		t.Fatalf("PhysicalContext = %#v, want empty when no context is supplied (AnalyzeEnvelope's existing behavior must be unchanged)", session.Evidence.PhysicalContext)
	}
}

// TestDiagnoseAnalysisWorksFromAnAlreadyPersistedAnalysisWithNoRawEnvelope
// is a direct regression test for a real bug found via manual testing: a
// stored MeasurementWindow whose Raw.SchemaVersion the current
// signalanalysis build no longer accepts (e.g. synthetic/demo data seeded
// with schema_version 0) must still be diagnosable from its own already-
// persisted Analysis -- DiagnoseAnalysis never touches Raw at all, so it
// can't fail this way.
func TestDiagnoseAnalysisWorksFromAnAlreadyPersistedAnalysisWithNoRawEnvelope(t *testing.T) {
	profile := pulseProfile(nil)
	analysis, err := signalanalysis.New().Analyze(pulseEnvelope(10, []float64{1, 1, 1}), profile)
	if err != nil {
		t.Fatalf("analyze reference: %v", err)
	}
	engine := NewEngine(signalanalysis.New())
	// No envelope is passed at all -- proving this path never depends on one.
	session, err := engine.DiagnoseAnalysis(context.Background(), profile, domain.StageDiagnose, "serial", analysis, nil, nil)
	if err != nil {
		t.Fatalf("DiagnoseAnalysis() error = %v", err)
	}
	if session.Diagnosis.Headline == "" {
		t.Fatalf("session = %#v", session)
	}
}

func TestAnalyzeSignalsRunsOnlyDeterministicAnalysis(t *testing.T) {
	engine := NewEngine(signalanalysis.New())
	profile := pulseProfile(nil)
	analysis, err := engine.AnalyzeSignals(pulseEnvelope(10, []float64{1, 1, 1}), profile)
	if err != nil {
		t.Fatalf("AnalyzeSignals() error = %v", err)
	}
	facts, ok := analysis.Probe("P2")
	if !ok || facts.FrequencyHz == nil {
		t.Fatalf("analysis = %#v, want a resolved frequency for the pulse probe", analysis)
	}
}

func pulseProfile(baseline *domain.TrustedBaseline) domain.ProjectProfile {
	return domain.ProjectProfile{
		ID:           "generic-pulse-test",
		ProjectName:  "Generic pulse test",
		Controller:   "Test controller",
		LogicVoltage: 3.3,
		Confirmed:    true,
		Probes: []domain.ProbeConfiguration{{
			Probe: "P2",
			Role:  "CLOCK",
			Mode:  domain.ProbeModePulse,
			Expected: domain.ExpectedSignal{
				SignalType:         "pulse",
				Required:           true,
				Stable:             true,
				MinFrequencyHz:     floatPointer(9),
				MaxFrequencyHz:     floatPointer(11),
				NominalFrequencyHz: floatPointer(10),
				MaxDropouts:        0,
			},
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1},
			Baseline:        baseline,
		}},
	}
}

func powerProfile(voltageRange []float64) domain.ProjectProfile {
	return domain.ProjectProfile{
		ID:           "power-rail-test",
		ProjectName:  "Power rail test",
		Controller:   "Test controller",
		LogicVoltage: 3.3,
		Confirmed:    true,
		Probes: []domain.ProbeConfiguration{{
			Probe:       "P1",
			Role:        "SUPPLY",
			Mode:        domain.ProbeModeAnalog,
			IsPowerRail: true,
			Expected: domain.ExpectedSignal{
				SignalType:          "voltage rail",
				Required:            true,
				Stable:              true,
				MinVoltage:          floatPointer(voltageRange[0]),
				MaxVoltage:          floatPointer(voltageRange[1]),
				VoltageTolerancePct: 2,
			},
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 2},
			Baseline:        &domain.TrustedBaseline{Status: domain.BaselineKnownGoodCapture, AverageVoltage: floatPointer(5)},
		}},
	}
}

func pulseEnvelope(rising uint32, activity []float64) domain.TelemetryEnvelope {
	return domain.TelemetryEnvelope{
		SchemaVersion: 2,
		DeviceID:      "test-device-001",
		ProfileID:     "generic-pulse-test",
		WindowMS:      1000,
		Samples:       []domain.TelemetrySample{pulseSample("P2", rising, activity)},
	}
}

func pulseSample(probe string, rising uint32, activity []float64) domain.TelemetrySample {
	periods := []float64{}
	widths := []float64{}
	if rising > 0 {
		periods = []float64{100_000, 100_000, 100_000}
		widths = []float64{50_000, 50_000, 50_000}
	}
	return domain.TelemetrySample{
		Probe:             probe,
		Mode:              domain.ProbeModePulse,
		EdgeCount:         rising * 2,
		RisingEdges:       rising,
		FallingEdges:      rising,
		PeriodsUS:         periods,
		HighPulseWidthsUS: widths,
		ActivityCounts:    activity,
	}
}

func analogEnvelope(millivolts []float64) domain.TelemetryEnvelope {
	return domain.TelemetryEnvelope{
		SchemaVersion: 2,
		DeviceID:      "test-device-001",
		ProfileID:     "power-rail-test",
		WindowMS:      1000,
		Samples: []domain.TelemetrySample{{
			Probe:    "P1",
			Mode:     domain.ProbeModeAnalog,
			AnalogMV: millivolts,
		}},
	}
}

func trustedFrequencyBaseline(frequency float64) *domain.TrustedBaseline {
	return &domain.TrustedBaseline{Status: domain.BaselineUserConfirmedHealthy, FrequencyHz: floatPointer(frequency), FrequencyTolerancePct: 5}
}

func floatPointer(value float64) *float64 { return &value }

func assertRule(t *testing.T, rules []domain.RuleResult, id, status string) {
	t.Helper()
	for _, rule := range rules {
		if rule.ID == id && rule.Status == status {
			return
		}
	}
	t.Fatalf("rules = %#v, want %s/%s", rules, id, status)
}
