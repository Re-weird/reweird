package testplanner

import (
	"fmt"
	"math"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func (planner *Planner) Evaluate(profile domain.ProjectProfile, plan domain.TestPlan, baseline, during domain.MeasurementWindow) domain.TestResult {
	result := domain.TestResult{
		TestID: plan.ID, TestType: plan.Recommendation.TestType,
		TargetProbes: append([]string(nil), plan.Recommendation.TargetProbes...),
		Observations: []domain.TestObservation{}, DerivedMetrics: map[string]any{},
		Result: "INCONCLUSIVE", Confidence: 0,
		EvidenceProvenance: []domain.Provenance{domain.ProvenanceMeasured, domain.ProvenanceDerived, domain.ProvenanceGuidedTest},
		TimestampMS:        time.Now().UTC().UnixMilli(),
	}
	if baseline.Analysis.ProfileID != profile.ID || during.Analysis.ProfileID != profile.ID || baseline.ID == 0 || during.ID == 0 {
		result.Interpretation = "Matching before and test measurement windows are required."
		return result
	}
	firstProbe := plan.Recommendation.TargetProbes[0]
	before, beforeOK := baseline.Analysis.Probe(firstProbe)
	current, currentOK := during.Analysis.Probe(firstProbe)
	if !beforeOK || !currentOK {
		result.Interpretation = "The target probe is absent from one or both windows."
		return result
	}
	add := func(probe, metric string, value any, unit string, provenance domain.Provenance) {
		result.Observations = append(result.Observations, domain.TestObservation{Probe: probe, Metric: metric, Value: value, Unit: unit, Provenance: provenance})
	}
	result.Confidence = 0.9
	switch plan.Recommendation.TestType {
	case domain.TestMovementCorrelation:
		beforeRate := dropoutRate(before.DropoutEvents, baseline.Analysis.WindowMS)
		duringRate := dropoutRate(current.DropoutEvents, during.Analysis.WindowMS)
		ratio := duringRate / math.Max(beforeRate, 1.0/60)
		result.DerivedMetrics["baseline_dropout_rate_per_minute"] = beforeRate
		result.DerivedMetrics["test_dropout_rate_per_minute"] = duringRate
		result.DerivedMetrics["change_ratio"] = ratio
		add(firstProbe, "baseline_dropouts", before.DropoutEvents, "events", domain.ProvenanceMeasured)
		add(firstProbe, "movement_dropouts", current.DropoutEvents, "events", domain.ProvenanceMeasured)
		if duringRate-beforeRate >= 2 && ratio >= 1.5 {
			result.Result = "POSITIVE_CORRELATION"
			result.Interpretation = "Signal failures increased substantially during the movement test; this supports correlation, not a specific physical cause."
		} else {
			result.Result = "NO_CORRELATION_OBSERVED"
			result.Interpretation = "This window did not show a substantial increase in failures during movement."
		}
	case domain.TestPowerRailStability:
		stableCount, unstableCount := 0, 0
		for _, probe := range plan.Recommendation.TargetProbes {
			configuration, _ := profile.Probe(probe)
			facts, ok := during.Analysis.Probe(probe)
			if !ok {
				result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "A monitored probe is absent from the test window.", 0
				return result
			}
			add(probe, "dropout_events", facts.DropoutEvents, "events", domain.ProvenanceDerived)
			if !configuration.IsPowerRail {
				continue
			}
			if facts.AverageVoltage == nil || facts.MinimumVoltage == nil || facts.MaximumVoltage == nil {
				result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "A configured rail needs voltage samples in the test window.", 0
				return result
			}
			add(probe, "average_voltage", *facts.AverageVoltage, "V", domain.ProvenanceDerived)
			add(probe, "minimum_voltage", *facts.MinimumVoltage, "V", domain.ProvenanceDerived)
			add(probe, "maximum_voltage", *facts.MaximumVoltage, "V", domain.ProvenanceDerived)
			add(probe, "voltage_variation", valueOrZero(facts.VoltageVariation), "V", domain.ProvenanceDerived)
			result.DerivedMetrics[probe+"_average_voltage"] = *facts.AverageVoltage
			if facts.Stable && voltageViolation(facts, configuration.Expected) == 0 {
				stableCount++
			} else {
				unstableCount++
			}
		}
		result.DerivedMetrics["simultaneous_failure_groups"] = during.Analysis.SimultaneousDropoutGroups
		if unstableCount > 0 {
			result.Result = "RAIL_OUTSIDE_TOLERANCE"
			result.Interpretation = "At least one configured power rail fell outside its expected range or stability tolerance."
		} else if stableCount > 0 {
			result.Result = "RAIL_STABLE"
			result.Interpretation = "The monitored power rail stayed inside its configured range and stability tolerance."
		}
	case domain.TestSimultaneousDropout:
		counts := map[int]int{}
		for _, probe := range plan.Recommendation.TargetProbes {
			facts, ok := during.Analysis.Probe(probe)
			if !ok {
				result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "A monitored probe is absent from the test window.", 0
				return result
			}
			for _, bucket := range facts.FailureBuckets {
				counts[bucket]++
			}
			add(probe, "failure_buckets", facts.FailureBuckets, "buckets", domain.ProvenanceDerived)
		}
		events := 0
		for _, count := range counts {
			if count >= 2 {
				events++
			}
		}
		result.DerivedMetrics["affected_probes"] = plan.Recommendation.TargetProbes
		result.DerivedMetrics["simultaneous_events"] = events
		if events > 0 {
			result.Result = "SHARED_FAILURE_PATTERN"
			result.Interpretation = "Multiple monitored probes failed in the same time buckets; the shared cause remains to be determined."
		} else {
			result.Result = "NO_SHARED_PATTERN_OBSERVED"
			result.Interpretation = "No matching failure buckets were measured in this window."
		}
	case domain.TestSignalActivity:
		configuration, _ := profile.Probe(firstProbe)
		add(firstProbe, "digital_transitions", current.DigitalTransitions, "edges", domain.ProvenanceMeasured)
		add(firstProbe, "pulse_count", current.PulseCount, "pulses", domain.ProvenanceMeasured)
		add(firstProbe, "expected_required", configuration.Expected.Required, "", domain.ProvenanceSpecification)
		result.DerivedMetrics["missing_expected_activity"] = current.MissingExpectedActivity
		if !configuration.Expected.Required {
			result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "The profile does not require activity on this probe.", 0
		} else if current.MissingExpectedActivity {
			result.Result = "EXPECTED_ACTIVITY_MISSING"
			result.Interpretation = "The required activity was absent in the captured window."
		} else {
			result.Result = "ACTIVITY_PRESENT"
			result.Interpretation = "The configured signal activity was present in the captured window."
		}
	case domain.TestFrequencyTiming:
		configuration, _ := profile.Probe(firstProbe)
		if current.FrequencyHz == nil {
			result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "No frequency could be derived from the captured window.", 0
			return result
		}
		add(firstProbe, "frequency", *current.FrequencyHz, "Hz", domain.ProvenanceDerived)
		for _, metric := range []struct {
			name string
			val  *float64
			unit string
		}{{"duty_cycle", current.DutyCyclePercent, "%"}, {"pulse_width", current.AveragePulseWidthUS, "us"}, {"jitter", current.JitterUS, "us"}} {
			if metric.val != nil {
				add(firstProbe, metric.name, *metric.val, metric.unit, domain.ProvenanceDerived)
				result.DerivedMetrics[metric.name] = *metric.val
			}
		}
		result.DerivedMetrics["frequency_hz"] = *current.FrequencyHz
		if configuration.Baseline != nil && configuration.Baseline.Status.Trusted() && current.BaselineDeviationPercent != nil {
			add(firstProbe, "trusted_baseline_deviation", *current.BaselineDeviationPercent, "%", domain.ProvenanceBaseline)
		}
		if frequencyOutside(*current.FrequencyHz, configuration.Expected) {
			result.Result = "TIMING_OUTSIDE_SPECIFICATION"
			result.Interpretation = "Measured frequency is outside the confirmed profile range."
		} else {
			result.Result = "TIMING_WITHIN_SPECIFICATION"
			result.Interpretation = "Measured frequency is inside the confirmed profile range."
		}
	case domain.TestBaselineComparison:
		for _, probe := range plan.Recommendation.TargetProbes {
			configuration, _ := profile.Probe(probe)
			if configuration.Baseline == nil || !configuration.Baseline.Status.Trusted() {
				result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "A trusted baseline is required for every target probe.", 0
				return result
			}
			facts, ok := during.Analysis.Probe(probe)
			if !ok || facts.BaselineDeviationPercent == nil {
				result.Result, result.Interpretation, result.Confidence = "INCONCLUSIVE", "No comparable trusted baseline metric was captured.", 0
				return result
			}
			add(probe, "baseline_deviation", *facts.BaselineDeviationPercent, "%", domain.ProvenanceBaseline)
			result.DerivedMetrics[probe+"_baseline_deviation_percent"] = *facts.BaselineDeviationPercent
			if *facts.BaselineDeviationPercent > baselineTolerance(configuration) {
				result.Result = "BASELINE_DEVIATION"
			}
		}
		if result.Result != "BASELINE_DEVIATION" {
			result.Result = "BASELINE_MATCH"
		}
		result.Interpretation = fmt.Sprintf("Compared captured values with trusted baselines: %s.", result.Result)
	case domain.TestRemeasure:
		comparison := planner.Verify(profile, plan.Recommendation.TargetProbes, baseline, during)
		result.Result = comparison.Status
		result.Interpretation = comparison.Summary
		result.DerivedMetrics["before_window_id"] = baseline.ID
		result.DerivedMetrics["test_window_id"] = during.ID
		if comparison.Status == "INCONCLUSIVE" {
			result.Confidence = 0
		}
	}
	return result
}

func dropoutRate(events int, windowMS uint32) float64 {
	if windowMS == 0 {
		return 0
	}
	return float64(events) * 60_000 / float64(windowMS)
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
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
