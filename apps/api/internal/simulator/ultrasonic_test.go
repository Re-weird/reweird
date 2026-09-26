package simulator

import (
	"context"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
)

func TestEveryScenarioTraversesRawTelemetryToExpectedRule(t *testing.T) {
	tests := []struct {
		scenario string
		rule     string
		status   string
	}{
		{ScenarioHealthy, "expected-activity", "pass"},
		{ScenarioDeadSignal, "missing-signal", "fail"},
		{ScenarioLowVoltage, "voltage-outside-specification", "fail"},
		{ScenarioUnstablePower, "power-rail-instability", "fail"},
		{ScenarioMissingPulses, "unexpected-dropout", "fail"},
		{ScenarioIntermittent, "unexpected-dropout", "fail"},
		{ScenarioSimultaneousDropouts, "simultaneous-dropout", "fail"},
		{ScenarioTimingDrift, "frequency-outside-specification", "fail"},
		{ScenarioSoftwareChange, "missing-signal", "fail"},
	}
	profile := profiles.UltrasonicDemo()
	engine := diagnostics.NewEngine(signalanalysis.New())
	for index, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			envelope := frameForScenario(test.scenario, domain.StageDiagnose, uint64(index+1))
			if err := telemetry.ValidateForProfile(envelope, profile); err != nil {
				t.Fatalf("ValidateForProfile() error = %v", err)
			}
			session, err := engine.AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "simulator", envelope, nil)
			if err != nil {
				t.Fatalf("AnalyzeEnvelope() error = %v", err)
			}
			if !containsRule(session.Evidence.RuleResults, test.rule, test.status) {
				t.Fatalf("rules = %#v, want %s/%s", session.Evidence.RuleResults, test.rule, test.status)
			}
		})
	}
}

func TestVerifyAlwaysProducesHealthyReMeasurement(t *testing.T) {
	profile := profiles.UltrasonicDemo()
	envelope := frameForScenario(ScenarioDeadSignal, domain.StageVerify, 2)
	analysis, err := signalanalysis.New().Analyze(envelope, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, facts := range analysis.Probes {
		if facts.Role != "UNASSIGNED" && !facts.Stable {
			t.Fatalf("%s remained unstable during VERIFY: %#v", facts.Probe, facts)
		}
	}
}

func containsRule(rules []domain.RuleResult, id, status string) bool {
	for _, rule := range rules {
		if rule.ID == id && rule.Status == status {
			return true
		}
	}
	return false
}
