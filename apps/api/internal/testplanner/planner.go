package testplanner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
)

type Planner struct{}

func New() *Planner { return &Planner{} }

// Recommend is a temporary deterministic adapter. A future provider can submit
// the same TestRecommendation to Plan without changing the workflow.
func (planner *Planner) Recommend(profile domain.ProjectProfile, analysis domain.AnalysisResult, sessionID string) domain.TestRecommendation {
	makeRecommendation := func(kind domain.TestType, probes []string, reason string, userAction bool) domain.TestRecommendation {
		return domain.TestRecommendation{
			ID: "recommended-" + string(kind), SessionID: sessionID,
			TestType: kind, TargetProbes: probes, Reason: reason,
			DurationSeconds: int(analysis.WindowMS / 1000), RequiresUserAction: userAction,
		}
	}
	// Loss of formerly confirmed activity outranks unrelated secondary drift.
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if ok && signalanalysis.KnownGoodActivityLost(facts, configuration) {
			recommendation := makeRecommendation(domain.TestRemeasure, []string{facts.Probe}, fmt.Sprintf("%s/%s has no valid activity compared with Known Good. Inspect the passive monitoring path, then capture fresh evidence; invalid edges do not establish a frequency.", facts.Probe, facts.Role), true)
			recommendation.Instructions = []string{"Inspect the passive probe connection against the confirmed probe plan; do not drive the target node or alter protection circuitry."}
			return recommendation
		}
	}
	// An unreliable capture must be fixed before any circuit test can be
	// interpreted; re-measure that probe instead of testing a hypothesis.
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if ok && configuration.Role != "UNASSIGNED" && facts.CaptureUnreliable {
			return makeRecommendation(domain.TestRemeasure, []string{facts.Probe}, "The raw capture is inconsistent; verify the probe connection and capture a new window before testing a cause.", false)
		}
	}
	for _, group := range analysis.SimultaneousDropoutGroups {
		if len(group) >= 2 {
			return makeRecommendation(domain.TestSimultaneousDropout, group, "Check whether monitored signals fail in the same time buckets.", false)
		}
	}
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if ok && configuration.IsPowerRail && !facts.Stable {
			return makeRecommendation(domain.TestPowerRailStability, []string{facts.Probe}, "Measure the configured power rail while monitoring other signal failures.", false)
		}
	}
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if !ok || configuration.Role == "UNASSIGNED" {
			continue
		}
		if facts.MissingExpectedActivity {
			return makeRecommendation(domain.TestSignalActivity, []string{facts.Probe}, "Check whether the expected signal activity is present.", false)
		}
		if facts.FrequencyHz != nil && frequencyOutside(*facts.FrequencyHz, configuration.Expected) || facts.PulseWidthOutOfRange {
			return makeRecommendation(domain.TestFrequencyTiming, []string{facts.Probe}, "Compare measured timing with the confirmed signal limits.", false)
		}
		if facts.DropoutEvents > configuration.Expected.MaxDropouts {
			return makeRecommendation(domain.TestMovementCorrelation, []string{facts.Probe}, "Check whether failures increase during controlled movement.", true)
		}
	}
	for _, configuration := range profile.Probes {
		facts, ok := analysis.Probe(configuration.Probe)
		if ok && configuration.Role != "UNASSIGNED" && len(facts.KnownGoodDeviations) > 0 {
			return makeRecommendation(domain.TestBaselineComparison, []string{facts.Probe}, "This signal left its confirmed Known Good envelope; compare it with the Known Good capture.", false)
		}
		if ok && configuration.Role != "UNASSIGNED" && facts.BaselineDeviationPercent != nil && configuration.Baseline != nil && configuration.Baseline.Status.Trusted() && configuration.Baseline.WindowCount == 0 {
			return makeRecommendation(domain.TestBaselineComparison, []string{facts.Probe}, "Compare this signal with its trusted healthy baseline.", false)
		}
	}
	var probes []string
	for _, configuration := range profile.Probes {
		if configuration.Role != "UNASSIGNED" && configuration.Expected.Required {
			probes = append(probes, configuration.Probe)
		}
	}
	return makeRecommendation(domain.TestRemeasure, probes, "Capture another window without presuming a fault.", false)
}

func (planner *Planner) Plan(profile domain.ProjectProfile, recommendation domain.TestRecommendation) (domain.TestPlan, error) {
	if !profile.Confirmed {
		return domain.TestPlan{}, errors.New("the Project Profile must be confirmed")
	}
	if recommendation.SessionID == "" || recommendation.Reason == "" {
		return domain.TestPlan{}, errors.New("session_id and reason are required")
	}
	if recommendation.DurationSeconds < 0 || recommendation.DurationSeconds > 60 {
		return domain.TestPlan{}, errors.New("duration_seconds must be between 0 and 60")
	}
	if recommendation.DurationSeconds == 0 {
		recommendation.DurationSeconds = 10
	}
	if len(recommendation.TargetProbes) == 0 || len(recommendation.TargetProbes) > 6 {
		return domain.TestPlan{}, errors.New("target_probes must contain 1-6 configured probes")
	}
	seen := make(map[string]bool)
	monitoring := make([]string, 0, len(recommendation.TargetProbes))
	for _, target := range recommendation.TargetProbes {
		configuration, ok := profile.Probe(target)
		if !ok || configuration.Role == "UNASSIGNED" || seen[target] {
			return domain.TestPlan{}, fmt.Errorf("%s must be a unique assigned probe in the confirmed profile", target)
		}
		seen[target] = true
		monitoring = append(monitoring, target+" — "+configuration.Role)
	}
	plan := domain.TestPlan{
		ID: recommendation.ID, Recommendation: recommendation,
		Monitoring: monitoring, WindowMS: uint32(recommendation.DurationSeconds * 1000),
		RequiresPatch: recommendation.RequiresPatch,
	}
	if recommendation.RequiresPatch {
		plan.Unavailable = "PATCH REQUIRED — Use the separate PATCH validation and approval flow. This passive measurement plan never drives hardware. Unprovisioned hardware remains locked."
	}
	base := []string{
		"Keep the project powered in its normal operating condition.",
		"Leave the passive ReWeird probes connected to the confirmed points.",
		"Capture a reference measurement window before the guided action.",
	}
	switch recommendation.TestType {
	case domain.TestMovementCorrelation:
		plan.Title = "Movement correlation test"
		plan.Metrics = []string{"dropout rate", "change ratio", "other signals active"}
		plan.Criteria = "Positive correlation requires at least two additional dropouts and a rate at least 1.5× the reference rate; it does not prove a loose wire."
		base = append(base, "When prompted, gently move the suspected connection while ReWeird records another window.", "Compare the dropout rates; do not disconnect a probe or apply an output signal.")
		plan.Recommendation.RequiresUserAction = true
	case domain.TestPowerRailStability:
		hasRail := false
		for _, target := range recommendation.TargetProbes {
			configuration, _ := profile.Probe(target)
			hasRail = hasRail || configuration.IsPowerRail
		}
		if !hasRail {
			return domain.TestPlan{}, errors.New("power-rail test requires a configured power-rail probe")
		}
		plan.Title = "Power rail stability test"
		plan.Metrics = []string{"average voltage", "minimum voltage", "maximum voltage", "variation", "dropout events", "shared failure buckets"}
		plan.Criteria = "Compare rail voltage and variation with the confirmed profile and check whether failures share time buckets."
		base = append(base, "Capture the rail during the reported behavior and compare with configured limits.")
	case domain.TestSimultaneousDropout:
		if len(recommendation.TargetProbes) < 2 {
			return domain.TestPlan{}, errors.New("simultaneous-dropout test requires at least two probes")
		}
		plan.Title = "Simultaneous dropout test"
		plan.Metrics = []string{"failure buckets", "affected probes", "simultaneous events"}
		plan.Criteria = "At least one matching failure bucket across two or more monitored probes indicates a shared failure pattern."
		base = append(base, "Capture all monitored signals in the same measurement window.")
	case domain.TestSignalActivity:
		plan.Title = "Signal activity test"
		plan.Metrics = []string{"digital state", "transitions", "pulse count", "missing expected activity"}
		plan.Criteria = "Required activity must be present according to the confirmed signal mode and expected behavior."
		base = append(base, "Run the project in the state where this signal is expected to be active.")
	case domain.TestFrequencyTiming:
		plan.Title = "Frequency and timing test"
		plan.Metrics = []string{"frequency", "duty cycle", "pulse width", "jitter", "trusted baseline deviation"}
		plan.Criteria = "Measured frequency must be inside configured limits; baseline evidence is used only when trusted."
		base = append(base, "Capture the signal while its timing source is operating normally.")
	case domain.TestBaselineComparison:
		for _, target := range recommendation.TargetProbes {
			configuration, _ := profile.Probe(target)
			if configuration.Baseline == nil || !configuration.Baseline.Status.Trusted() {
				return domain.TestPlan{}, fmt.Errorf("%s has no trusted baseline", target)
			}
		}
		plan.Title = "Trusted baseline comparison"
		plan.Metrics = []string{"voltage", "frequency", "dropouts", "variation", "baseline deviation"}
		plan.Criteria = "Compare only with a user-confirmed, known-good, or manufacturer baseline."
		base = append(base, "Capture another measurement and compare only available trusted baseline metrics.")
	case domain.TestRemeasure:
		plan.Title = "Re-measure"
		plan.Metrics = []string{"expected activity", "dropouts", "stability", "voltage", "frequency"}
		plan.Criteria = "Compare the new window with the previous window and all applicable profile limits."
		base = append(base, "Capture another measurement window after the reported condition or user change.")
	default:
		return domain.TestPlan{}, fmt.Errorf("unsupported test_type %q", recommendation.TestType)
	}
	plan.Instructions = append(base, recommendation.Instructions...)
	plan.Instructions = append(plan.Instructions, "Review structured evidence and re-measure after any correction.")
	if strings.TrimSpace(plan.ID) == "" {
		return domain.TestPlan{}, errors.New("recommendation id is required")
	}
	return plan, nil
}

func frequencyOutside(value float64, expected domain.ExpectedSignal) bool {
	return (expected.MinFrequencyHz != nil && value < *expected.MinFrequencyHz) ||
		(expected.MaxFrequencyHz != nil && value > *expected.MaxFrequencyHz)
}
