package diagnostics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/probe"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
)

type Engine struct {
	analyzer *signalanalysis.Analyzer
	probe    probe.Provider
}

func NewEngine(analyzer *signalanalysis.Analyzer) *Engine {
	if analyzer == nil {
		analyzer = signalanalysis.New()
	}
	return &Engine{analyzer: analyzer, probe: probe.MockProvider{}}
}

// NewEngineWithProbe wires a PROBE interpretation provider (e.g. Gemini)
// into the engine. The provider only rewords the deterministic diagnosis;
// it never sees raw telemetry and cannot change measured evidence.
func NewEngineWithProbe(analyzer *signalanalysis.Analyzer, provider probe.Provider) *Engine {
	engine := NewEngine(analyzer)
	if provider != nil {
		engine.probe = provider
	}
	return engine
}

func (engine *Engine) Analyze(
	ctx context.Context,
	profile domain.ProjectProfile,
	stage domain.Stage,
	source domain.TelemetrySource,
	reference *domain.AnalysisResult,
) (domain.Session, error) {
	envelope, err := source.Latest(ctx)
	if err != nil {
		return domain.Session{}, fmt.Errorf("read %s telemetry: %w", source.Name(), err)
	}
	return engine.AnalyzeEnvelope(ctx, profile, stage, source.Name(), envelope, reference)
}

func (engine *Engine) AnalyzeEnvelope(
	ctx context.Context,
	profile domain.ProjectProfile,
	stage domain.Stage,
	telemetryMode string,
	envelope domain.TelemetryEnvelope,
	reference *domain.AnalysisResult,
) (domain.Session, error) {
	return engine.AnalyzeEnvelopeWithContext(ctx, profile, stage, telemetryMode, envelope, reference, nil)
}

// AnalyzeSignals runs only the deterministic signal-analysis step (no
// rules, no diagnosis, no PROBE) against an already-received envelope. It
// exists so a caller that ingests telemetry outside the built-in
// TelemetrySource loop (e.g. a per-project measurement submitted over HTTP)
// can reuse the exact same analyzer this engine already uses, rather than
// constructing a second one.
func (engine *Engine) AnalyzeSignals(envelope domain.TelemetryEnvelope, profile domain.ProjectProfile) (domain.AnalysisResult, error) {
	return engine.analyzer.Analyze(envelope, profile)
}

// AnalyzeEnvelopeWithContext is AnalyzeEnvelope plus a bounded slice of
// supplementary, already-provenance-tagged EvidenceFacts (e.g. from
// internal/diagnosticcontext) merged into the returned Evidence before
// PROBE ever sees it. Passing nil is exactly AnalyzeEnvelope's behavior --
// this engine still never reads a repository directly; the caller is
// responsible for building physicalContext from persisted state and
// keeping it small.
func (engine *Engine) AnalyzeEnvelopeWithContext(
	ctx context.Context,
	profile domain.ProjectProfile,
	stage domain.Stage,
	telemetryMode string,
	envelope domain.TelemetryEnvelope,
	reference *domain.AnalysisResult,
	physicalContext []domain.EvidenceFact,
) (domain.Session, error) {
	analysis, err := engine.analyzer.Analyze(envelope, profile)
	if err != nil {
		return domain.Session{}, fmt.Errorf("analyze telemetry: %w", err)
	}
	session, err := engine.DiagnoseAnalysis(ctx, profile, stage, telemetryMode, analysis, reference, physicalContext)
	if err != nil {
		return domain.Session{}, err
	}
	// DiagnoseAnalysis itself never requires a raw envelope (that's the
	// point of it -- see its own comment), so it never sets RawTelemetry.
	// This caller has one, so it audits it here: unchanged raw edges stay
	// available separately from the validated derived facts in Evidence.
	session.RawTelemetry = envelope
	return session, nil
}

// DiagnoseAnalysis is AnalyzeEnvelopeWithContext for a caller that already
// has a persisted, previously-computed domain.AnalysisResult (e.g. a stored
// MeasurementWindow's own Analysis field) and should not re-run signal
// analysis against its raw envelope -- re-deriving would needlessly require
// that historical envelope to still match this build's current
// signalanalysis schema/version expectations, when the already-persisted
// analysis is exactly what was already trusted and shown at the time it was
// captured. This is the same "derive from what's already persisted, don't
// recompute" pattern Physical Restore/Verify already use for their own
// evidence.
func (engine *Engine) DiagnoseAnalysis(
	ctx context.Context,
	profile domain.ProjectProfile,
	stage domain.Stage,
	telemetryMode string,
	analysis domain.AnalysisResult,
	reference *domain.AnalysisResult,
	physicalContext []domain.EvidenceFact,
) (domain.Session, error) {
	rules := evaluateRules(profile, analysis, stage, reference)
	focus := selectFocus(profile, analysis, rules, reference)
	diagnosis := buildDiagnosis(profile, analysis, rules, focus, stage)
	evidence := buildEvidence(profile, analysis, rules, focus, stage, reference)
	if len(physicalContext) > 0 {
		evidence.PhysicalContext = physicalContext
	}
	diagnosis = engine.probe.Diagnose(ctx, evidence, diagnosis)
	if focus.CaptureUnreliable {
		diagnosis.Confidence = math.Min(diagnosis.Confidence, 0.5)
	}

	beforeFacts := focus
	if reference != nil {
		if previous, ok := reference.Probe(focus.Probe); ok {
			beforeFacts = previous
		}
	}
	before := domain.MeasurementSummary{DropoutsPerMinute: beforeFacts.DropoutEvents, Stability: stabilityLabel(beforeFacts)}
	var after *domain.MeasurementSummary
	if stage == domain.StageVerify {
		after = &domain.MeasurementSummary{DropoutsPerMinute: focus.DropoutEvents, Stability: stabilityLabel(focus)}
	}

	return domain.Session{
		ID:                "session-" + profile.ID,
		ProjectName:       profile.ProjectName,
		Stage:             stage,
		HardwareConnected: true,
		TelemetryMode:     telemetryMode,
		ProfileID:         profile.ID,
		Probes:            probeReadings(profile, analysis, focus.Probe, stage),
		Analysis:          analysis,
		Evidence:          evidence,
		Diagnosis:         diagnosis,
		Before:            before,
		After:             after,
		Timeline:          timeline(stage, focus),
	}, nil
}

func evaluateRules(profile domain.ProjectProfile, analysis domain.AnalysisResult, stage domain.Stage, reference *domain.AnalysisResult) []domain.RuleResult {
	var results []domain.RuleResult
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if !ok || configuration.Role == "UNASSIGNED" {
			continue
		}

		if signalanalysis.KnownGoodActivityLost(facts, configuration) {
			results = append(results, rule("known-good-activity-lost", facts.Probe, "fail", 6,
				fmt.Sprintf("%s activity missing or unreliable compared with Known Good; verify the passive monitoring connection before concluding the target signal failed", facts.Role)))
		}
		if facts.CaptureUnreliable {
			results = append(results, rule("capture-unreliable", facts.Probe, "fail", 5,
				fmt.Sprintf("%s measurement is unreliable: %s", facts.Role, strings.Join(facts.CaptureIssues, "; "))))
			continue // invalid edges cannot support electrical/timing fault rules
		}

		if facts.MissingExpectedActivity {
			results = append(results, rule("missing-signal", facts.Probe, "fail", 3,
				fmt.Sprintf("%s has no expected activity", facts.Role)))
		}

		if facts.PulseWidthOutOfRange {
			results = append(results, rule("pulse-width-outside-specification", facts.Probe, "fail", 3,
				fmt.Sprintf("%s pulse width %s is outside the configured %s", facts.Role, widthRange(facts.MinimumPulseWidthUS, facts.MaximumPulseWidthUS), widthRange(configuration.Expected.MinPulseWidthUS, configuration.Expected.MaxPulseWidthUS))))
		}

		for _, deviation := range facts.KnownGoodDeviations {
			results = append(results, rule("known-good-deviation", facts.Probe, "fail", 3,
				fmt.Sprintf("%s %s", facts.Role, deviation)))
		}

		if facts.AverageVoltage != nil && (configuration.Expected.MinVoltage != nil || configuration.Expected.MaxVoltage != nil) {
			outside := (configuration.Expected.MinVoltage != nil && *facts.AverageVoltage < *configuration.Expected.MinVoltage) ||
				(configuration.Expected.MaxVoltage != nil && *facts.AverageVoltage > *configuration.Expected.MaxVoltage)
			if outside {
				results = append(results, rule("voltage-outside-specification", facts.Probe, "fail", 3,
					fmt.Sprintf("%s is outside its trusted voltage specification at %.2f V", facts.Role, *facts.AverageVoltage)))
			} else if configuration.IsPowerRail {
				results = append(results, rule("power-rail-stability", facts.Probe, "pass", 0,
					fmt.Sprintf("%s rail is stable at %.2f V", facts.Role, *facts.AverageVoltage)))
			}
		}

		if configuration.IsPowerRail && !facts.Stable {
			results = append(results, rule("power-rail-instability", facts.Probe, "fail", 4,
				fmt.Sprintf("%s rail variation exceeds the configured tolerance", facts.Role)))
		}

		if facts.FrequencyHz != nil && (configuration.Expected.MinFrequencyHz != nil || configuration.Expected.MaxFrequencyHz != nil) {
			outside := (configuration.Expected.MinFrequencyHz != nil && *facts.FrequencyHz < *configuration.Expected.MinFrequencyHz) ||
				(configuration.Expected.MaxFrequencyHz != nil && *facts.FrequencyHz > *configuration.Expected.MaxFrequencyHz)
			if outside {
				results = append(results, rule("frequency-outside-specification", facts.Probe, "fail", 2,
					fmt.Sprintf("%s frequency %.2f Hz is outside the configured range", facts.Role, *facts.FrequencyHz)))
			} else if facts.DropoutEvents == 0 && !facts.MissingExpectedActivity {
				results = append(results, rule("expected-activity", facts.Probe, "pass", 0,
					fmt.Sprintf("%s activity is present at %s", facts.Role, formatFrequency(*facts.FrequencyHz))))
			}
		}

		if facts.DropoutEvents > configuration.Expected.MaxDropouts {
			results = append(results, rule("unexpected-dropout", facts.Probe, "fail", 3,
				fmt.Sprintf("%d unexpected %s dropouts detected", facts.DropoutEvents, facts.Role)))
		} else if stage == domain.StageVerify && configuration.Expected.Required {
			results = append(results, rule("unexpected-dropout", facts.Probe, "pass", 0,
				fmt.Sprintf("%s has no unexpected dropouts after repair", facts.Role)))
		}

		if facts.BaselineDeviationPercent != nil && configuration.Baseline != nil && configuration.Baseline.Status.Trusted() && configuration.Baseline.WindowCount == 0 {
			tolerance := baselineTolerance(configuration)
			if *facts.BaselineDeviationPercent > tolerance {
				results = append(results, rule("baseline-deviation", facts.Probe, "warn", 2,
					fmt.Sprintf("%s differs from its trusted baseline by %.1f%%", facts.Role, *facts.BaselineDeviationPercent)))
			} else if stage == domain.StageVerify {
				results = append(results, rule("baseline-deviation", facts.Probe, "pass", 0,
					fmt.Sprintf("%s matches its trusted healthy baseline", facts.Role)))
			}
		}
	}

	for _, group := range analysis.SimultaneousDropoutGroups {
		results = append(results, rule("simultaneous-dropout", strings.Join(group, ","), "fail", 4,
			fmt.Sprintf("Simultaneous dropouts detected on %s; investigate a shared power or ground cause", strings.Join(group, ", "))))
	}

	if stage == domain.StageTest && reference != nil {
		for _, current := range analysis.Probes {
			previous, ok := reference.Probe(current.Probe)
			if !ok || current.CaptureUnreliable || previous.CaptureUnreliable || current.DropoutEvents < previous.DropoutEvents+2 {
				continue
			}
			ratio := float64(current.DropoutEvents) / math.Max(1, float64(previous.DropoutEvents))
			if ratio >= 1.5 {
				results = append(results, rule("movement-correlation", current.Probe, "fail", 4,
					fmt.Sprintf("%s dropout rate increased from %d to %d during movement; evidence strongly supports an intermittent physical connection on this signal path", current.Role, previous.DropoutEvents, current.DropoutEvents)))
			}
		}
	}

	if stage == domain.StageDiagnose {
		for _, facts := range analysis.Probes {
			if facts.DropoutEvents > 0 {
				results = append(results, rule("movement-correlation", facts.Probe, "warn", 1,
					fmt.Sprintf("Movement correlation has not been tested for %s", facts.Role)))
				break
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Severity > results[j].Severity
	})
	return results
}

func selectFocus(profile domain.ProjectProfile, analysis domain.AnalysisResult, rules []domain.RuleResult, reference *domain.AnalysisResult) domain.DerivedFacts {
	for _, result := range rules {
		if result.Status != "pass" && !strings.Contains(result.Probe, ",") {
			if facts, ok := analysis.Probe(result.Probe); ok {
				return facts
			}
		}
	}
	if reference != nil {
		var referenceFocus domain.DerivedFacts
		for _, previous := range reference.Probes {
			if previous.DropoutEvents > referenceFocus.DropoutEvents || previous.MissingExpectedActivity {
				referenceFocus = previous
			}
		}
		if referenceFocus.Probe != "" {
			if current, ok := analysis.Probe(referenceFocus.Probe); ok {
				return current
			}
		}
	}
	for _, configuration := range profile.Probes {
		if configuration.Expected.Required {
			if facts, ok := analysis.Probe(configuration.Probe); ok {
				return facts
			}
		}
	}
	if len(analysis.Probes) > 0 {
		return analysis.Probes[0]
	}
	return domain.DerivedFacts{Probe: "P1", Role: "UNASSIGNED"}
}

func buildDiagnosis(profile domain.ProjectProfile, analysis domain.AnalysisResult, rules []domain.RuleResult, focus domain.DerivedFacts, stage domain.Stage) domain.Diagnosis {
	if hasRuleForProbe(rules, "known-good-activity-lost", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s activity missing or unreliable compared with Known Good", focus.Role),
			Summary:        fmt.Sprintf("No valid %s pulse train can be established on %s. Known Good recorded periodic activity. Unmatched/glitch edges are retained as raw evidence, not treated as a measured rate. This does not distinguish a detached passive probe from an inactive target signal.", focus.Role, focus.Probe),
			PossibleCauses: []string{"Disconnected or floating passive monitoring probe", "Unreliable capture or input level", "Target signal inactive"},
			Confidence:     0.5,
			NextTest:       fmt.Sprintf("Inspect the passive %s/%s monitoring connection against the confirmed probe plan, then re-measure; do not drive the node.", focus.Probe, focus.Role),
		}
	}
	if stage == domain.StageVerify && focus.Stable && !hasFailure(rules) {
		return domain.Diagnosis{
			Headline:       "Issue resolved",
			Summary:        fmt.Sprintf("The %s signal is stable after repair and matches the configured healthy behavior.", focus.Role),
			PossibleCauses: []string{"Previously intermittent physical connection"},
			Confidence:     0.98,
			NextTest:       "Continue monitoring during normal operation.",
		}
	}
	if hasRuleForProbe(rules, "capture-unreliable", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s measurement is not trustworthy", focus.Role),
			Summary:        fmt.Sprintf("The raw %s capture on %s is internally inconsistent, so it cannot support a circuit diagnosis. The raw values are kept unchanged; fix the measurement path before interpreting this signal.", focus.Role, focus.Probe),
			PossibleCauses: []string{"Pulses shorter than the input capture can resolve", "Probe level below the input logic threshold", "Probe not attached to the expected node", "Firmware capture path mismatch"},
			Confidence:     0.4,
			NextTest:       fmt.Sprintf("Verify the %s probe connection and level, then capture a new window.", focus.Probe),
		}
	}
	if hasRule(rules, "movement-correlation", "fail") {
		return domain.Diagnosis{
			Headline:       "Movement correlation confirmed",
			Summary:        fmt.Sprintf("%s failures increased during movement while the other monitored paths remained active. Evidence strongly supports an intermittent physical connection on this signal path.", focus.Role),
			PossibleCauses: []string{"Intermittent jumper wire", "Loose connector or breadboard contact", "Poor contact on the monitored signal path"},
			Confidence:     0.92,
			NextTest:       fmt.Sprintf("Reseat or replace the %s connection, then re-measure.", focus.Role),
		}
	}
	if hasRuleForProbe(rules, "missing-signal", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("Missing %s activity", focus.Role),
			Summary:        "The configured signal should be active, but no transitions or pulses were measured in the current window.",
			PossibleCauses: []string{"Inactive source", "Open signal path", "Incorrect probe assignment", "Software did not command the expected state"},
			Confidence:     0.78,
			NextTest:       "Confirm the probe assignment, then compare the physical signal with the software command state.",
		}
	}
	if hasRuleForProbe(rules, "voltage-outside-specification", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s voltage outside specification", focus.Role),
			Summary:        "The measured voltage is outside a trusted range from the confirmed Project Profile.",
			PossibleCauses: []string{"Supply regulation problem", "Unexpected load", "Incorrect divider scale", "Wiring resistance"},
			Confidence:     0.9,
			NextTest:       "Verify the measurement divider and compare the source rail under load.",
		}
	}
	if hasRuleForProbe(rules, "frequency-outside-specification", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s timing outside specification", focus.Role),
			Summary:        "The signal remains active, but its measured frequency is outside the confirmed Project Profile range.",
			PossibleCauses: []string{"Clock or timer configuration drift", "Unexpected software timing change", "Oscillator or supply instability"},
			Confidence:     0.88,
			NextTest:       "Compare the configured timer or PWM settings with the measured frequency and jitter.",
		}
	}
	if hasRuleForProbe(rules, "pulse-width-outside-specification", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s pulse width outside specification", focus.Role),
			Summary:        "The signal is active, but its measured HIGH time is outside the configured pulse-width limits.",
			PossibleCauses: []string{"Software timing change", "Load or level distortion on the signal path", "Component responding differently"},
			Confidence:     0.85,
			NextTest:       "Compare the commanded pulse timing in software with the measured pulse width.",
		}
	}
	if hasRule(rules, "simultaneous-dropout", "fail") || hasRule(rules, "power-rail-instability", "fail") {
		return domain.Diagnosis{
			Headline:       "Shared electrical instability",
			Summary:        "Several monitored paths failed together or the configured power rail moved outside tolerance. Investigate a shared supply or ground cause first.",
			PossibleCauses: []string{"Unstable power supply", "Shared ground problem", "Connector or harness affecting multiple paths"},
			Confidence:     0.82,
			NextTest:       "Measure the configured power rail and ground reference during the failure window.",
		}
	}
	if hasRuleForProbe(rules, "known-good-deviation", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("%s deviates from Known Good", focus.Role),
			Summary:        fmt.Sprintf("%s is within its configured limits but outside the physical behavior you confirmed as healthy for this device and profile revision.", focus.Role),
			PossibleCauses: []string{"Degrading connection or component", "Changed load or operating condition", "Software behavior changed since calibration"},
			Confidence:     0.75,
			NextTest:       fmt.Sprintf("Compare %s against its Known Good capture while reproducing the reported condition.", focus.Probe),
		}
	}
	if hasRuleForProbe(rules, "unexpected-dropout", focus.Probe) {
		return domain.Diagnosis{
			Headline:       fmt.Sprintf("Intermittent %s activity", focus.Role),
			Summary:        fmt.Sprintf("The failure is isolated to the %s path. Current evidence does not yet prove a loose connection.", focus.Role),
			PossibleCauses: []string{"Intermittent connection", "Component malfunction", "Software-controlled switching"},
			Confidence:     0.68,
			NextTest:       fmt.Sprintf("Gently wiggle the %s connection while %s is monitored.", focus.Role, focus.Probe),
		}
	}
	return domain.Diagnosis{
		Headline:       "Signals within configured limits",
		Summary:        fmt.Sprintf("No deterministic rule found a fault in the current %s measurement window.", profile.ProjectName),
		PossibleCauses: []string{},
		Confidence:     0.8,
		NextTest:       "Continue monitoring or reproduce the reported failure condition.",
	}
}

func buildEvidence(profile domain.ProjectProfile, analysis domain.AnalysisResult, rules []domain.RuleResult, focus domain.DerivedFacts, stage domain.Stage, reference *domain.AnalysisResult) domain.Evidence {
	configuration, _ := profile.Probe(focus.Probe)
	expected := map[string]any{
		"signal":               configuration.Expected.SignalType,
		"required":             configuration.Expected.Required,
		"max_dropouts":         configuration.Expected.MaxDropouts,
		"safe_max_pin_voltage": configuration.SafeMeasurement.MaxPinVoltage,
	}
	if configuration.Expected.MinVoltage != nil {
		expected["min_voltage"] = *configuration.Expected.MinVoltage
	}
	if configuration.Expected.MaxVoltage != nil {
		expected["max_voltage"] = *configuration.Expected.MaxVoltage
	}
	if configuration.Expected.NominalFrequencyHz != nil {
		expected["nominal_frequency_hz"] = *configuration.Expected.NominalFrequencyHz
	}
	if configuration.Expected.MinFrequencyHz != nil {
		expected["min_frequency_hz"] = *configuration.Expected.MinFrequencyHz
	}
	if configuration.Expected.MaxFrequencyHz != nil {
		expected["max_frequency_hz"] = *configuration.Expected.MaxFrequencyHz
	}
	if configuration.Expected.MinPulseWidthUS != nil {
		expected["min_pulse_width_us"] = *configuration.Expected.MinPulseWidthUS
	}
	if configuration.Expected.MaxPulseWidthUS != nil {
		expected["max_pulse_width_us"] = *configuration.Expected.MaxPulseWidthUS
	}

	observed := map[string]any{
		"pulse_detected":            !focus.CaptureUnreliable && focus.PulseCount > 0,
		"dropouts_per_window":       focus.DropoutEvents,
		"missing_expected_activity": focus.MissingExpectedActivity,
		"stable":                    focus.Stable,
		"capture_reliable":          !focus.CaptureUnreliable,
		"rail_voltage_stable":       powerRailStable(profile, analysis),
		"other_signals_active":      otherSignalsActive(profile, analysis, focus.Probe),
		"movement_correlation":      hasRuleForProbe(rules, "movement-correlation", focus.Probe) && stage == domain.StageTest,
	}
	measurements := []domain.EvidenceFact{
		{Probe: focus.Probe, Name: "pulse_count", Value: focus.PulseCount, Provenance: domain.ProvenanceMeasured},
		{Probe: focus.Probe, Name: "digital_transitions", Value: focus.DigitalTransitions, Provenance: domain.ProvenanceMeasured},
	}
	derived := []domain.EvidenceFact{
		{Probe: focus.Probe, Name: "dropout_events", Value: focus.DropoutEvents, Provenance: domain.ProvenanceDerived},
		{Probe: focus.Probe, Name: "stable", Value: focus.Stable, Provenance: domain.ProvenanceDerived},
	}
	if focus.AverageVoltage != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "average_voltage", Value: *focus.AverageVoltage, Unit: "V", Provenance: domain.ProvenanceDerived})
	}
	if focus.FrequencyHz != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "frequency", Value: *focus.FrequencyHz, Unit: "Hz", Provenance: domain.ProvenanceDerived})
	}
	if focus.DutyCyclePercent != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "duty_cycle", Value: *focus.DutyCyclePercent, Unit: "%", Provenance: domain.ProvenanceDerived})
	}
	if focus.JitterUS != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "jitter", Value: *focus.JitterUS, Unit: "us", Provenance: domain.ProvenanceDerived})
	}
	if focus.AveragePulseWidthUS != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "average_pulse_width", Value: *focus.AveragePulseWidthUS, Unit: "us", Provenance: domain.ProvenanceDerived})
	}
	if focus.MaximumGapUS != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "maximum_gap", Value: *focus.MaximumGapUS, Unit: "us", Provenance: domain.ProvenanceDerived})
	}
	if focus.RailStable != nil {
		derived = append(derived, domain.EvidenceFact{Probe: focus.Probe, Name: "rail_stable", Value: *focus.RailStable, Provenance: domain.ProvenanceDerived})
	}

	baseline := map[string]any{"status": domain.BaselineUnknown}
	var baselineFacts []domain.EvidenceFact
	var unresolved []string
	if configuration.Baseline != nil {
		baseline["status"] = configuration.Baseline.Status
		baseline["trusted"] = configuration.Baseline.Status.Trusted()
		if configuration.Baseline.FrequencyHz != nil {
			baseline["frequency_hz"] = *configuration.Baseline.FrequencyHz
		}
		if configuration.Baseline.AverageVoltage != nil {
			baseline["average_voltage"] = *configuration.Baseline.AverageVoltage
		}
		if configuration.Baseline.WindowCount > 0 {
			baseline["learned_windows"] = configuration.Baseline.WindowCount
		}
		if configuration.Baseline.MinFrequencyHz != nil && configuration.Baseline.MaxFrequencyHz != nil {
			baseline["frequency_range_hz"] = fmt.Sprintf("%.3f–%.3f", *configuration.Baseline.MinFrequencyHz, *configuration.Baseline.MaxFrequencyHz)
		}
		if configuration.Baseline.MinPulseWidthUS != nil && configuration.Baseline.MaxPulseWidthUS != nil {
			baseline["pulse_width_range_us"] = fmt.Sprintf("%.1f–%.1f", *configuration.Baseline.MinPulseWidthUS, *configuration.Baseline.MaxPulseWidthUS)
		}
		for _, deviation := range focus.KnownGoodDeviations {
			baselineFacts = append(baselineFacts, domain.EvidenceFact{Probe: focus.Probe, Name: "known_good_deviation", Value: true, Provenance: domain.ProvenanceBaseline, Detail: deviation})
		}
		if focus.BaselineDeviationPercent != nil {
			baselineFacts = append(baselineFacts, domain.EvidenceFact{Probe: focus.Probe, Name: "baseline_deviation", Value: *focus.BaselineDeviationPercent, Unit: "%", Provenance: domain.ProvenanceBaseline})
		}
	}
	if configuration.Baseline == nil || !configuration.Baseline.Status.Trusted() {
		unresolved = append(unresolved, fmt.Sprintf("No trusted healthy baseline exists for %s", focus.Role))
	}
	if focus.CaptureUnreliable {
		unresolved = append(unresolved, fmt.Sprintf("Can %s be captured reliably? The current raw capture is inconsistent.", focus.Probe))
	}
	if stage == domain.StageDiagnose && focus.DropoutEvents > 0 {
		unresolved = append(unresolved, "Does the dropout rate increase during controlled movement?")
	}
	if reference == nil && stage == domain.StageTest {
		unresolved = append(unresolved, "Movement correlation requires a pre-test reference window")
	}

	specificationFacts := make([]domain.EvidenceFact, 0)
	for _, result := range rules {
		if result.Probe == focus.Probe && (result.ID == "voltage-outside-specification" || result.ID == "frequency-outside-specification") {
			specificationFacts = append(specificationFacts, domain.EvidenceFact{Probe: focus.Probe, Name: result.ID, Value: result.Status, Provenance: domain.ProvenanceSpecification, Detail: result.Message})
		}
	}

	return domain.Evidence{
		Probe:                focus.Probe,
		Role:                 focus.Role,
		Expected:             expected,
		Observed:             observed,
		Baseline:             baseline,
		Measurements:         measurements,
		DerivedFacts:         derived,
		SpecificationResults: specificationFacts,
		BaselineComparison:   baselineFacts,
		RuleResults:          rules,
		UnresolvedQuestions:  unresolved,
	}
}

func probeReadings(profile domain.ProjectProfile, analysis domain.AnalysisResult, focusProbe string, stage domain.Stage) []domain.ProbeReading {
	readings := make([]domain.ProbeReading, 0, len(profile.Probes))
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if !ok {
			continue
		}
		value, unit := displayValue(facts)
		status := "idle"
		if configuration.Role != "UNASSIGNED" {
			if !facts.Stable {
				status = "intermittent"
			} else if facts.Mode == domain.ProbeModeAnalog || (stage == domain.StageVerify && facts.Probe == focusProbe) {
				status = "stable"
			} else {
				status = "active"
			}
		}
		readings = append(readings, domain.ProbeReading{
			Probe:    facts.Probe,
			Role:     configuration.Role,
			Value:    value,
			Unit:     unit,
			Status:   status,
			Dropouts: facts.DropoutEvents,
			Samples:  append([]float64(nil), facts.ActivityCounts...),
		})
	}
	return readings
}

func displayValue(facts domain.DerivedFacts) (*float64, string) {
	if facts.CaptureUnreliable {
		return nil, "unreliable"
	}
	if facts.AverageVoltage != nil {
		return facts.AverageVoltage, "V"
	}
	if facts.FrequencyHz != nil {
		if *facts.FrequencyHz >= 1000 {
			value := *facts.FrequencyHz / 1000
			return &value, "kHz"
		}
		return facts.FrequencyHz, "pulses/s"
	}
	if facts.DigitalState != nil {
		value := float64(*facts.DigitalState)
		return &value, ""
	}
	return nil, ""
}

func timeline(stage domain.Stage, focus domain.DerivedFacts) []domain.TimelineEvent {
	tested := stage == domain.StageTest || stage == domain.StageRepair || stage == domain.StageVerify
	verified := stage == domain.StageVerify
	return []domain.TimelineEvent{
		{ID: "detect", Label: "Detect", Detail: fmt.Sprintf("%s anomaly detected", focus.Role), Time: "00:04", Complete: true},
		{ID: "diagnose", Label: "Diagnose", Detail: fmt.Sprintf("Evidence isolated to %s", focus.Probe), Time: "00:07", Complete: true},
		{ID: "test", Label: "Test", Detail: choose(tested, "Movement correlation evaluated", "Wiggle test ready"), Time: "00:12", Complete: tested},
		{ID: "verify", Label: "Verify", Detail: choose(verified, "Signal compared with baseline", "Waiting for repair"), Time: "00:18", Complete: verified},
	}
}

func rule(id, probe, status string, severity int, message string) domain.RuleResult {
	provenance := domain.ProvenanceDerived
	if id == "voltage-outside-specification" || id == "frequency-outside-specification" {
		provenance = domain.ProvenanceSpecification
	}
	if id == "baseline-deviation" {
		provenance = domain.ProvenanceBaseline
	}
	return domain.RuleResult{ID: id, Probe: probe, Status: status, Severity: severity, Message: message, Provenance: provenance}
}

func hasRule(results []domain.RuleResult, id, status string) bool {
	for _, result := range results {
		if result.ID == id && result.Status == status {
			return true
		}
	}
	return false
}

func hasRuleForProbe(results []domain.RuleResult, id, probe string) bool {
	for _, result := range results {
		if result.ID == id && result.Probe == probe && result.Status != "pass" {
			return true
		}
	}
	return false
}

func hasFailure(results []domain.RuleResult) bool {
	for _, result := range results {
		if result.Status == "fail" {
			return true
		}
	}
	return false
}

func baselineTolerance(configuration domain.ProbeConfiguration) float64 {
	if configuration.Baseline == nil {
		return math.Inf(1)
	}
	if configuration.Baseline.VoltageTolerancePct > 0 {
		return configuration.Baseline.VoltageTolerancePct
	}
	if configuration.Baseline.FrequencyTolerancePct > 0 {
		return configuration.Baseline.FrequencyTolerancePct
	}
	return 10
}

func powerRailStable(profile domain.ProjectProfile, analysis domain.AnalysisResult) bool {
	for _, configuration := range profile.Probes {
		if !configuration.IsPowerRail {
			continue
		}
		facts, ok := analysis.Probe(configuration.Probe)
		if !ok || !facts.Stable {
			return false
		}
	}
	return true
}

func otherSignalsActive(profile domain.ProjectProfile, analysis domain.AnalysisResult, excluded string) bool {
	for _, configuration := range profile.Probes {
		if configuration.Probe == excluded || !configuration.Expected.Required {
			continue
		}
		facts, ok := analysis.Probe(configuration.Probe)
		if !ok || facts.MissingExpectedActivity || !facts.Stable {
			return false
		}
	}
	return true
}

func formatFrequency(frequency float64) string {
	if frequency >= 1000 {
		return fmt.Sprintf("%.1f kHz", frequency/1000)
	}
	return fmt.Sprintf("%.1f Hz", frequency)
}

func stabilityLabel(facts domain.DerivedFacts) string {
	if facts.Stable {
		return "stable"
	}
	return "intermittent"
}

func widthRange(minimum, maximum *float64) string {
	switch {
	case minimum != nil && maximum != nil:
		return fmt.Sprintf("%.1f–%.1f µs", *minimum, *maximum)
	case minimum != nil:
		return fmt.Sprintf("≥ %.1f µs", *minimum)
	case maximum != nil:
		return fmt.Sprintf("≤ %.1f µs", *maximum)
	default:
		return "unspecified"
	}
}

func choose(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}
