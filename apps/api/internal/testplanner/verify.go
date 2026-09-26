package testplanner

import (
	"fmt"
	"math"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// Verify compares two independently captured windows against confirmed profile
// limits. A changed value alone is never evidence that a fault was resolved.
func (planner *Planner) Verify(profile domain.ProjectProfile, targets []string, before, after domain.MeasurementWindow) domain.VerificationResult {
	result := domain.VerificationResult{
		Status: "INCONCLUSIVE", Improvements: []domain.MetricChange{}, RemainingIssues: []string{},
		Changes: []domain.MetricChange{}, BeforeWindowID: before.ID, AfterWindowID: after.ID,
		TimestampMS: time.Now().UTC().UnixMilli(),
	}
	if !profile.Confirmed || before.ID == 0 || after.ID == 0 || before.ID == after.ID ||
		before.Analysis.ProfileID != profile.ID || after.Analysis.ProfileID != profile.ID || len(targets) == 0 {
		result.Summary = "Two distinct measurement windows matching a confirmed Project Profile are required."
		return result
	}
	beforeSeverity, afterSeverity, criteria := 0.0, 0.0, 0
	for _, target := range targets {
		config, configured := profile.Probe(target)
		old, oldOK := before.Analysis.Probe(target)
		newFacts, newOK := after.Analysis.Probe(target)
		if !configured || !oldOK || !newOK {
			result.RemainingIssues = append(result.RemainingIssues, target+": comparable probe evidence is missing")
			result.Summary = "At least one target probe was not measured in both windows."
			return result
		}
		record := func(name string, oldValue, newValue float64, unit string, lowerIsBetter bool) {
			if math.Abs(oldValue-newValue) < 1e-9 {
				return
			}
			change := domain.MetricChange{Probe: target, Metric: name, Before: oldValue, After: newValue, Unit: unit}
			result.Changes = append(result.Changes, change)
			if (lowerIsBetter && newValue < oldValue) || (!lowerIsBetter && newValue > oldValue) {
				result.Improvements = append(result.Improvements, change)
			}
		}
		record("dropout_rate_per_minute", dropoutRate(old.DropoutEvents, before.Analysis.WindowMS), dropoutRate(newFacts.DropoutEvents, after.Analysis.WindowMS), "events/min", true)
		if config.Expected.Required || config.Expected.MaxDropouts > 0 {
			criteria++
			oldExcess := math.Max(0, dropoutRate(old.DropoutEvents, before.Analysis.WindowMS)-dropoutRate(config.Expected.MaxDropouts, before.Analysis.WindowMS))
			newExcess := math.Max(0, dropoutRate(newFacts.DropoutEvents, after.Analysis.WindowMS)-dropoutRate(config.Expected.MaxDropouts, after.Analysis.WindowMS))
			beforeSeverity += oldExcess
			afterSeverity += newExcess
			if newExcess > 0 {
				result.RemainingIssues = append(result.RemainingIssues, fmt.Sprintf("%s: dropout rate exceeds the profile limit", target))
			}
		}
		if config.Expected.Required {
			criteria++
			if old.MissingExpectedActivity {
				beforeSeverity += 10
			}
			if newFacts.MissingExpectedActivity {
				afterSeverity += 10
				result.RemainingIssues = append(result.RemainingIssues, target+": required activity is still missing")
			}
		}
		if config.Expected.Stable || config.IsPowerRail {
			criteria++
			if !old.Stable {
				beforeSeverity += 5
			}
			if !newFacts.Stable {
				afterSeverity += 5
				result.RemainingIssues = append(result.RemainingIssues, target+": signal remains unstable")
			}
		}
		if old.AverageVoltage != nil && newFacts.AverageVoltage != nil {
			record("average_voltage", *old.AverageVoltage, *newFacts.AverageVoltage, "V", false)
		}
		if old.MinimumVoltage != nil && newFacts.MinimumVoltage != nil {
			record("minimum_voltage", *old.MinimumVoltage, *newFacts.MinimumVoltage, "V", false)
		}
		if old.MaximumVoltage != nil && newFacts.MaximumVoltage != nil {
			record("maximum_voltage", *old.MaximumVoltage, *newFacts.MaximumVoltage, "V", false)
		}
		if config.Expected.MinVoltage != nil || config.Expected.MaxVoltage != nil {
			if old.MinimumVoltage == nil || old.MaximumVoltage == nil || newFacts.MinimumVoltage == nil || newFacts.MaximumVoltage == nil {
				result.Summary = "Required voltage evidence was unavailable in one or both windows."
				return result
			}
			criteria++
			oldBad, newBad := voltageViolation(old, config.Expected), voltageViolation(newFacts, config.Expected)
			beforeSeverity += oldBad
			afterSeverity += newBad
			if newBad > 0 {
				result.RemainingIssues = append(result.RemainingIssues, target+": voltage remains outside configured limits")
			}
		}
		if old.VoltageVariation != nil && newFacts.VoltageVariation != nil {
			record("voltage_variation", *old.VoltageVariation, *newFacts.VoltageVariation, "V", true)
		}
		if old.FrequencyHz != nil && newFacts.FrequencyHz != nil {
			record("frequency", *old.FrequencyHz, *newFacts.FrequencyHz, "Hz", false)
		}
		if config.Expected.MinFrequencyHz != nil || config.Expected.MaxFrequencyHz != nil {
			criteria++
			oldBad, newBad := 0.0, 0.0
			if old.FrequencyHz == nil {
				oldBad = 5
			} else {
				oldBad = rangeViolation(*old.FrequencyHz, config.Expected.MinFrequencyHz, config.Expected.MaxFrequencyHz)
			}
			if newFacts.FrequencyHz == nil {
				newBad = 5
			} else {
				newBad = rangeViolation(*newFacts.FrequencyHz, config.Expected.MinFrequencyHz, config.Expected.MaxFrequencyHz)
			}
			beforeSeverity += oldBad
			afterSeverity += newBad
			if newBad > 0 {
				result.RemainingIssues = append(result.RemainingIssues, target+": expected frequency is missing or outside configured limits")
			}
		}
		if old.DutyCyclePercent != nil && newFacts.DutyCyclePercent != nil {
			record("duty_cycle", *old.DutyCyclePercent, *newFacts.DutyCyclePercent, "%", false)
		}
		if old.AveragePulseWidthUS != nil && newFacts.AveragePulseWidthUS != nil {
			record("pulse_width", *old.AveragePulseWidthUS, *newFacts.AveragePulseWidthUS, "us", false)
		}
		if old.JitterUS != nil && newFacts.JitterUS != nil {
			record("jitter", *old.JitterUS, *newFacts.JitterUS, "us", true)
		}
		if config.Baseline != nil && config.Baseline.Status.Trusted() && old.BaselineDeviationPercent != nil && newFacts.BaselineDeviationPercent != nil {
			criteria++
			record("trusted_baseline_deviation", *old.BaselineDeviationPercent, *newFacts.BaselineDeviationPercent, "%", true)
			limit := baselineTolerance(config)
			beforeSeverity += math.Max(0, *old.BaselineDeviationPercent-limit)
			afterSeverity += math.Max(0, *newFacts.BaselineDeviationPercent-limit)
			if *newFacts.BaselineDeviationPercent > limit {
				result.RemainingIssues = append(result.RemainingIssues, target+": trusted baseline deviation remains high")
			}
		}
	}
	if criteria == 0 {
		result.Summary = "The profile has no applicable confirmed limits for these probes."
		return result
	}
	switch {
	case beforeSeverity > 0 && afterSeverity == 0:
		result.Status, result.Summary = "RESOLVED", "Previously abnormal measurements now satisfy all applicable confirmed profile limits."
	case afterSeverity < beforeSeverity:
		result.Status, result.Summary = "IMPROVED", "Measurements improved, but at least one configured issue remains."
	case afterSeverity > beforeSeverity:
		result.Status, result.Summary = "WORSE", "Measurements moved further outside applicable profile limits."
	default:
		result.Status, result.Summary = "UNCHANGED", "No verified improvement against applicable profile limits was measured."
	}
	return result
}

func voltageViolation(facts domain.DerivedFacts, expected domain.ExpectedSignal) float64 {
	if facts.MinimumVoltage == nil || facts.MaximumVoltage == nil {
		return 1
	}
	return rangeViolation(*facts.MinimumVoltage, expected.MinVoltage, nil) + rangeViolation(*facts.MaximumVoltage, nil, expected.MaxVoltage)
}

func rangeViolation(value float64, minimum, maximum *float64) float64 {
	bad := 0.0
	if minimum != nil && value < *minimum {
		bad += *minimum - value
	}
	if maximum != nil && value > *maximum {
		bad += value - *maximum
	}
	return bad
}
