package signalanalysis

import (
	"fmt"
	"math"
	"sort"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

func (analyzer *Analyzer) Analyze(envelope domain.TelemetryEnvelope, profile domain.ProjectProfile) (domain.AnalysisResult, error) {
	if err := telemetry.ValidateForProfile(envelope, profile); err != nil {
		return domain.AnalysisResult{}, err
	}

	result := domain.AnalysisResult{
		SchemaVersion: envelope.SchemaVersion,
		DeviceID:      envelope.DeviceID,
		ProfileID:     envelope.ProfileID,
		CapturedAtMS:  envelope.CapturedAtMS,
		WindowMS:      envelope.WindowMS,
		Probes:        make([]domain.DerivedFacts, 0, len(envelope.Samples)),
	}
	for _, sample := range envelope.Samples {
		configuration, _ := profile.Probe(sample.Probe)
		facts := analyzeSample(sample, configuration, envelope.WindowMS)
		result.Probes = append(result.Probes, facts)
	}
	result.SimultaneousDropoutGroups = simultaneousGroups(result.Probes)
	return result, nil
}

func analyzeSample(sample domain.TelemetrySample, configuration domain.ProbeConfiguration, windowMS uint32) domain.DerivedFacts {
	facts := domain.DerivedFacts{
		Probe:              sample.Probe,
		Role:               configuration.Role,
		Mode:               sample.Mode,
		DigitalState:       sample.State,
		DigitalTransitions: sample.EdgeCount,
		PulseCount:         sample.RisingEdges,
		ActivityCounts:     append([]float64(nil), sample.ActivityCounts...),
		Stable:             true,
	}

	if len(sample.AnalogMV) > 0 {
		scale := configuration.SafeMeasurement.InputScale
		if scale <= 0 {
			scale = 1
		}
		voltages := make([]float64, len(sample.AnalogMV))
		for index, millivolts := range sample.AnalogMV {
			voltages[index] = millivolts / 1000 * scale
		}
		minimum, maximum := bounds(voltages)
		average := mean(voltages)
		variation := maximum - minimum
		facts.AverageVoltage = pointer(average)
		facts.MinimumVoltage = pointer(minimum)
		facts.MaximumVoltage = pointer(maximum)
		facts.VoltageVariation = pointer(variation)
	}

	if len(sample.PeriodsUS) > 0 {
		averagePeriod := mean(sample.PeriodsUS)
		if averagePeriod > 0 {
			facts.FrequencyHz = pointer(1_000_000 / averagePeriod)
		}
		jitter := standardDeviation(sample.PeriodsUS)
		facts.JitterUS = pointer(jitter)
	} else if sample.RisingEdges > 0 && windowMS > 0 {
		frequency := float64(sample.RisingEdges) / (float64(windowMS) / 1000)
		facts.FrequencyHz = pointer(frequency)
	}

	if len(sample.HighPulseWidthsUS) > 0 && len(sample.PeriodsUS) > 0 {
		minimum, maximum := bounds(sample.HighPulseWidthsUS)
		average := mean(sample.HighPulseWidthsUS)
		facts.AveragePulseWidthUS = pointer(average)
		facts.MinimumPulseWidthUS = pointer(minimum)
		facts.MaximumPulseWidthUS = pointer(maximum)
		period := mean(sample.PeriodsUS)
		if period > 0 {
			duty := mean(sample.HighPulseWidthsUS) / period * 100
			facts.DutyCyclePercent = pointer(math.Min(100, duty))
		}
	}
	if sample.MaxGapUS > 0 {
		facts.MaximumGapUS = pointer(float64(sample.MaxGapUS))
	}

	for index, activity := range sample.ActivityCounts {
		if activity == 0 && configuration.Expected.Required {
			facts.FailureBuckets = append(facts.FailureBuckets, index)
		}
	}

	facts.DropoutEvents = expectedDropouts(sample, configuration.Expected, windowMS)
	if len(facts.FailureBuckets) > facts.DropoutEvents {
		facts.DropoutEvents = len(facts.FailureBuckets)
	}
	facts.MissingExpectedActivity = missingActivity(facts, configuration.Expected)
	facts.Stable = determineStability(facts, configuration)
	if configuration.IsPowerRail {
		stable := facts.Stable
		facts.RailStable = &stable
	}
	facts.BaselineDeviationPercent = baselineDeviation(facts, configuration.Baseline)
	return facts
}

func expectedDropouts(sample domain.TelemetrySample, expected domain.ExpectedSignal, windowMS uint32) int {
	gapDropouts := 0
	if expected.NominalFrequencyHz != nil && *expected.NominalFrequencyHz > 0 && sample.MaxGapUS > 0 {
		expectedPeriod := 1_000_000 / *expected.NominalFrequencyHz
		gapDropouts = int(math.Max(0, math.Floor(float64(sample.MaxGapUS)/expectedPeriod)-1))
	}
	if expected.NominalFrequencyHz == nil || windowMS == 0 || !expected.Required {
		return maxInt(len(zeroBuckets(sample.ActivityCounts)), gapDropouts)
	}
	expectedPulses := int(math.Round(*expected.NominalFrequencyHz * float64(windowMS) / 1000))
	observedPulses := int(sample.RisingEdges)
	if observedPulses >= expectedPulses {
		return gapDropouts
	}
	return maxInt(expectedPulses-observedPulses, gapDropouts)
}

func missingActivity(facts domain.DerivedFacts, expected domain.ExpectedSignal) bool {
	if !expected.Required {
		return false
	}
	switch facts.Mode {
	case domain.ProbeModeAnalog:
		return facts.AverageVoltage == nil || *facts.AverageVoltage <= 0.01
	case domain.ProbeModeDigital, domain.ProbeModePulse:
		return facts.DigitalTransitions == 0 && facts.PulseCount == 0
	default:
		return true
	}
}

func determineStability(facts domain.DerivedFacts, configuration domain.ProbeConfiguration) bool {
	if facts.MissingExpectedActivity || facts.DropoutEvents > configuration.Expected.MaxDropouts {
		return false
	}
	if facts.AverageVoltage != nil {
		if configuration.Expected.MinVoltage != nil && *facts.AverageVoltage < *configuration.Expected.MinVoltage {
			return false
		}
		if configuration.Expected.MaxVoltage != nil && *facts.AverageVoltage > *configuration.Expected.MaxVoltage {
			return false
		}
		if configuration.Expected.Stable && facts.VoltageVariation != nil {
			tolerance := configuration.Expected.VoltageTolerancePct
			if tolerance <= 0 {
				tolerance = 5
			}
			if *facts.AverageVoltage > 0 && (*facts.VoltageVariation / *facts.AverageVoltage * 100) > tolerance {
				return false
			}
		}
	}
	return true
}

func baselineDeviation(facts domain.DerivedFacts, baseline *domain.TrustedBaseline) *float64 {
	if baseline == nil || !baseline.Status.Trusted() {
		return nil
	}
	if facts.AverageVoltage != nil && baseline.AverageVoltage != nil && *baseline.AverageVoltage != 0 {
		deviation := math.Abs(*facts.AverageVoltage-*baseline.AverageVoltage) / math.Abs(*baseline.AverageVoltage) * 100
		return pointer(deviation)
	}
	if facts.FrequencyHz != nil && baseline.FrequencyHz != nil && *baseline.FrequencyHz != 0 {
		deviation := math.Abs(*facts.FrequencyHz-*baseline.FrequencyHz) / math.Abs(*baseline.FrequencyHz) * 100
		return pointer(deviation)
	}
	return nil
}

func simultaneousGroups(probes []domain.DerivedFacts) [][]string {
	byBucket := make(map[int][]string)
	for _, facts := range probes {
		for _, bucket := range facts.FailureBuckets {
			byBucket[bucket] = append(byBucket[bucket], facts.Probe)
		}
	}
	unique := make(map[string][]string)
	for _, probesAtBucket := range byBucket {
		if len(probesAtBucket) < 2 {
			continue
		}
		sort.Strings(probesAtBucket)
		key := fmt.Sprint(probesAtBucket)
		unique[key] = probesAtBucket
	}
	groups := make([][]string, 0, len(unique))
	for _, group := range unique {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return fmt.Sprint(groups[i]) < fmt.Sprint(groups[j]) })
	return groups
}

func zeroBuckets(values []float64) []int {
	var buckets []int
	for index, value := range values {
		if value == 0 {
			buckets = append(buckets, index)
		}
	}
	return buckets
}

func bounds(values []float64) (float64, float64) {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	return minimum, maximum
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func standardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	average := mean(values)
	var squared float64
	for _, value := range values {
		delta := value - average
		squared += delta * delta
	}
	return math.Sqrt(squared / float64(len(values)))
}

func pointer(value float64) *float64 { return &value }

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
