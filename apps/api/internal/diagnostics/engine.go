package diagnostics

import "github.com/re-weird/reweird/apps/api/internal/domain"

type Engine struct {
	telemetry domain.TelemetrySource
}

func NewEngine(telemetry domain.TelemetrySource) *Engine {
	return &Engine{telemetry: telemetry}
}

func (engine *Engine) Analyze(stage domain.Stage) domain.Session {
	probes := engine.telemetry.Snapshot(stage)
	verified := stage == domain.StageVerify
	wiggling := stage == domain.StageTest || stage == domain.StageRepair
	dropouts := probes[2].Dropouts

	rules := []domain.RuleResult{
		{ID: "power-rail", Status: "pass", Message: "Power rail is stable at 5.01 V"},
		{ID: "trigger-active", Status: "pass", Message: "TRIG activity is present at 40 kHz"},
	}
	if verified {
		rules = append(rules,
			domain.RuleResult{ID: "echo-dropouts", Status: "pass", Message: "ECHO is stable after repair"},
			domain.RuleResult{ID: "movement-correlation", Status: "pass", Message: "No movement-correlated failures remain"},
		)
	} else {
		rules = append(rules, domain.RuleResult{ID: "echo-dropouts", Status: "fail", Message: messageForDropouts(dropouts)})
		if wiggling {
			rules = append(rules, domain.RuleResult{ID: "movement-correlation", Status: "fail", Message: "Dropout rate increased during movement"})
		} else {
			rules = append(rules, domain.RuleResult{ID: "movement-correlation", Status: "warn", Message: "Movement correlation has not been tested"})
		}
	}

	diagnosis := initialDiagnosis()
	if wiggling {
		diagnosis = testDiagnosis()
	}
	if verified {
		diagnosis = verifiedDiagnosis()
	}

	var after *domain.MeasurementSummary
	if verified {
		after = &domain.MeasurementSummary{DropoutsPerMinute: 0, Stability: "stable"}
	}

	return domain.Session{
		ID:                "demo-ultrasonic-001",
		ProjectName:       "Ultrasonic Distance Sensor",
		Stage:             stage,
		HardwareConnected: true,
		Probes:            probes,
		Evidence: domain.Evidence{
			Probe: "P3",
			Role:  "ECHO",
			Expected: map[string]any{"signal": "return pulse", "voltage": "3.3V logic", "dropouts_per_minute": 0},
			Observed: map[string]any{"pulse_detected": true, "dropouts_per_minute": dropouts, "rail_voltage_stable": true, "other_signals_active": true, "movement_correlation": wiggling},
			Baseline: map[string]any{"dropouts_per_minute": 0, "average_pulses_per_second": 28.4},
			RuleResults: rules,
		},
		Diagnosis: diagnosis,
		Before:    domain.MeasurementSummary{DropoutsPerMinute: 12, Stability: "intermittent"},
		After:     after,
		Timeline: timeline(wiggling, verified),
	}
}

func messageForDropouts(count int) string {
	if count == 27 {
		return "27 unexpected ECHO dropouts detected"
	}
	return "12 unexpected ECHO dropouts detected"
}

func initialDiagnosis() domain.Diagnosis {
	return domain.Diagnosis{
		Headline: "Intermittent ECHO activity",
		Summary: "The failure is isolated to the ECHO path. Current evidence does not yet prove a loose connection.",
		PossibleCauses: []string{"Intermittent connection", "Sensor malfunction", "Software-controlled switching"},
		Confidence: 0.68,
		NextTest: "Gently wiggle the ECHO jumper while P3 is monitored.",
	}
}

func testDiagnosis() domain.Diagnosis {
	return domain.Diagnosis{
		Headline: "Movement correlation confirmed",
		Summary: "ECHO failures repeatedly increased while the connection was moved. Power and TRIG remained stable.",
		PossibleCauses: []string{"Intermittent jumper wire", "Loose breadboard contact", "Poor ECHO pin connection"},
		Confidence: 0.92,
		NextTest: "Reseat or replace the ECHO jumper, then re-measure.",
	}
}

func verifiedDiagnosis() domain.Diagnosis {
	return domain.Diagnosis{
		Headline: "Issue resolved",
		Summary: "The ECHO signal is stable after the simulated repair and now matches its healthy baseline.",
		PossibleCauses: []string{"Previously intermittent ECHO connection"},
		Confidence: 0.98,
		NextTest: "Continue monitoring during normal operation.",
	}
}

func timeline(tested, verified bool) []domain.TimelineEvent {
	return []domain.TimelineEvent{
		{ID: "detect", Label: "Detect", Detail: "Unexpected ECHO dropouts", Time: "00:04", Complete: true},
		{ID: "diagnose", Label: "Diagnose", Detail: "Fault isolated to P3", Time: "00:07", Complete: true},
		{ID: "test", Label: "Test", Detail: choose(tested || verified, "Movement correlation found", "Wiggle test ready"), Time: "00:12", Complete: tested || verified},
		{ID: "verify", Label: "Verify", Detail: choose(verified, "Signal matches baseline", "Waiting for repair"), Time: "00:18", Complete: verified},
	}
}

func choose(condition bool, yes, no string) string {
	if condition { return yes }
	return no
}
