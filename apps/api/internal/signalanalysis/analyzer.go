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
		SchemaVersion:  envelope.SchemaVersion,
		DeviceID:       envelope.DeviceID,
		ProfileID:      envelope.ProfileID,
		ProfileVersion: profile.Version,
		CapturedAtMS:   envelope.CapturedAtMS,
		WindowMS:       envelope.WindowMS,
		Probes:         make([]domain.DerivedFacts, 0, len(envelope.Samples)),
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
	expected := configuration.Expected

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

	gapTrusted := true
	if sample.Mode != domain.ProbeModeAnalog {
		gapTrusted = checkCaptureConsistency(sample, windowMS, &facts)
		checkPulseValidity(sample, configuration, windowMS, &facts)
	}

	if !facts.CaptureUnreliable && len(sample.PeriodsUS) > 0 {
		averagePeriod := mean(sample.PeriodsUS)
		if averagePeriod > 0 {
			facts.FrequencyHz = pointer(1_000_000 / averagePeriod)
		}
		jitter := standardDeviation(sample.PeriodsUS)
		facts.JitterUS = pointer(jitter)
	} else if !facts.CaptureUnreliable && sample.RisingEdges > 0 && windowMS > 0 {
		frequency := float64(sample.RisingEdges) / (float64(windowMS) / 1000)
		facts.FrequencyHz = pointer(frequency)
	}

	if !facts.CaptureUnreliable && len(sample.HighPulseWidthsUS) > 0 {
		minimum, maximum := bounds(sample.HighPulseWidthsUS)
		average := mean(sample.HighPulseWidthsUS)
		facts.AveragePulseWidthUS = pointer(average)
		facts.MinimumPulseWidthUS = pointer(minimum)
		facts.MaximumPulseWidthUS = pointer(maximum)
		if len(sample.PeriodsUS) > 0 {
			period := mean(sample.PeriodsUS)
			if period > 0 {
				duty := average / period * 100
				facts.DutyCyclePercent = pointer(math.Min(100, duty))
			}
		}
		if expected.MinPulseWidthUS != nil && minimum < *expected.MinPulseWidthUS ||
			expected.MaxPulseWidthUS != nil && maximum > *expected.MaxPulseWidthUS {
			facts.PulseWidthOutOfRange = true
		}
	}
	if sample.MaxGapUS > 0 {
		facts.MaximumGapUS = pointer(float64(sample.MaxGapUS))
	}

	if sample.Mode != domain.ProbeModeAnalog && !facts.CaptureUnreliable {
		if bucketRuleApplies(expected, windowMS, len(sample.ActivityCounts)) {
			for index, activity := range sample.ActivityCounts {
				if activity == 0 {
					facts.FailureBuckets = append(facts.FailureBuckets, index)
				}
			}
		}
		facts.DropoutEvents = expectedDropouts(sample, expected, windowMS, gapTrusted)
		if len(facts.FailureBuckets) > facts.DropoutEvents {
			facts.DropoutEvents = len(facts.FailureBuckets)
		}
	}
	facts.MissingExpectedActivity = missingActivity(facts, expected)
	facts.Stable = determineStability(facts, configuration)
	if configuration.IsPowerRail {
		stable := facts.Stable
		facts.RailStable = &stable
	}
	facts.BaselineDeviationPercent = baselineDeviation(facts, configuration.Baseline)
	facts.KnownGoodDeviations = KnownGoodDeviations(facts, configuration.Baseline)
	return facts
}

// Aggregate timing arrays cannot safely be re-paired after dropping glitches.
// Reject derived timing for the whole window, retaining the original envelope.
// The quarter-width floor is deliberately below healthy/spec widths: ordinary
// timing drift still remains measurable and is handled by specification rules.
func checkPulseValidity(sample domain.TelemetrySample, configuration domain.ProbeConfiguration, windowMS uint32, facts *domain.DerivedFacts) {
	flag := func(issue string) {
		facts.CaptureUnreliable = true
		facts.CaptureIssues = append(facts.CaptureIssues, issue)
	}
	if sample.RisingEdges > 0 && sample.FallingEdges == 0 {
		flag("no completed HIGH pulse: rising edges without falling edges; timing withheld")
	}
	if len(sample.PeriodsUS) > 0 && len(sample.HighPulseWidthsUS) == 0 {
		flag("period samples have no completed HIGH pulse measurements; timing withheld")
	}
	floor := 0.0
	useFloor := func(value *float64) {
		if value != nil && *value > 0 && (floor == 0 || *value/4 < floor) {
			floor = *value / 4
		}
	}
	useFloor(configuration.Expected.MinPulseWidthUS)
	if baseline := configuration.Baseline; baseline != nil && baseline.Status.Trusted() {
		useFloor(baseline.MinPulseWidthUS)
	}
	if len(sample.PeriodsUS) >= 2 && windowMS > 0 &&
		(slowestExpectedRate(configuration.Expected) > 0 || configuration.Baseline != nil) &&
		mean(sample.PeriodsUS)*(float64(sample.RisingEdges)+1) < float64(windowMS)*100 {
		flag("sampled periods describe a short burst inconsistent with the window edge count; sustained frequency withheld")
	}
	for _, width := range sample.HighPulseWidthsUS {
		if width <= 0 || (floor > 0 && width < floor) {
			flag(fmt.Sprintf("HIGH pulse %.3f us is below the minimum-valid pulse floor %.3f us; possible glitch, timing withheld", width, floor))
			break
		}
	}
}

// No valid periodic activity can be established where confirmed Known Good had
// activity. This does not prove that the target circuit stopped: a detached
// passive probe or unreliable input capture can produce the same observation.
func KnownGoodActivityLost(facts domain.DerivedFacts, configuration domain.ProbeConfiguration) bool {
	b := configuration.Baseline
	return configuration.Expected.Required && b != nil && b.Status.Trusted() && b.WindowCount > 0 &&
		((b.FrequencyHz != nil && *b.FrequencyHz > 0) || (b.MinFrequencyHz != nil && *b.MinFrequencyHz > 0)) &&
		facts.FrequencyHz == nil && (facts.CaptureUnreliable || facts.MissingExpectedActivity)
}

// checkCaptureConsistency flags raw captures that are internally
// inconsistent. It never edits the raw values; it only reports that they
// cannot support a circuit conclusion. It returns false when max_gap_us is
// not a trustworthy in-window measurement.
func checkCaptureConsistency(sample domain.TelemetrySample, windowMS uint32, facts *domain.DerivedFacts) bool {
	gapTrusted := true
	windowUS := float64(windowMS) * 1000
	// The device measures the window with millis() and edges with a
	// microsecond timer; allow one millisecond of clock-domain slack.
	limitUS := windowUS + 1000
	flag := func(issue string) {
		facts.CaptureUnreliable = true
		facts.CaptureIssues = append(facts.CaptureIssues, issue)
	}
	if sample.EdgeCount != sample.RisingEdges+sample.FallingEdges {
		flag(fmt.Sprintf("edge_count %d does not equal %d rising + %d falling edges", sample.EdgeCount, sample.RisingEdges, sample.FallingEdges))
	}
	if difference := int64(sample.RisingEdges) - int64(sample.FallingEdges); difference > 1 || difference < -1 {
		flag(fmt.Sprintf("%d rising vs %d falling edges: edges were missed or misclassified by the input capture", sample.RisingEdges, sample.FallingEdges))
	}
	if windowMS > 0 {
		if float64(sample.MaxGapUS) > limitUS {
			gapTrusted = false
			flag(fmt.Sprintf("max_gap_us %d exceeds the %d ms capture window", sample.MaxGapUS, windowMS))
		}
		if exceeds(sample.PeriodsUS, limitUS) || exceeds(sample.HighPulseWidthsUS, limitUS) {
			flag("pulse timing spans more than one capture window")
		}
	}
	if len(sample.HighPulseWidthsUS) > int(sample.FallingEdges) || len(sample.PeriodsUS) > int(sample.RisingEdges) {
		flag("more pulse timings than captured edges")
	}
	return gapTrusted
}

func exceeds(values []float64, limit float64) bool {
	for _, value := range values {
		if value > limit {
			return true
		}
	}
	return false
}

// slowestExpectedRate is the lowest pulse rate the configuration still
// considers healthy. MinFrequencyHz wins because it is the explicit bound;
// a nominal value alone has no tolerance and is used only as a fallback.
func slowestExpectedRate(expected domain.ExpectedSignal) float64 {
	if expected.MinFrequencyHz != nil && *expected.MinFrequencyHz > 0 {
		return *expected.MinFrequencyHz
	}
	if expected.NominalFrequencyHz != nil && *expected.NominalFrequencyHz > 0 {
		return *expected.NominalFrequencyHz
	}
	return 0
}

// bucketRuleApplies reports whether an empty activity bucket is evidence of
// a dropout. That is only true when even the slowest healthy rate must put
// at least one pulse in every bucket; a 4 Hz signal legitimately leaves most
// 100 ms buckets empty.
func bucketRuleApplies(expected domain.ExpectedSignal, windowMS uint32, buckets int) bool {
	if !expected.Required || buckets == 0 || windowMS == 0 {
		return false
	}
	rate := slowestExpectedRate(expected)
	if rate <= 0 {
		return false
	}
	bucketSeconds := float64(windowMS) / 1000 / float64(buckets)
	return rate*bucketSeconds >= 1
}

// expectedDropouts counts pulses that should have occurred but did not. The
// reference period is the signal's own regular rhythm inside this window
// when one is measurable (a gap several periods long is a missed pulse),
// otherwise the longest period the configuration still considers healthy.
// Only periodic, required signals can have dropouts.
func expectedDropouts(sample domain.TelemetrySample, expected domain.ExpectedSignal, windowMS uint32, gapTrusted bool) int {
	if !expected.Required || windowMS == 0 {
		return 0
	}
	rate := slowestExpectedRate(expected)
	if rate <= 0 {
		return 0
	}
	referencePeriodUS := 1_000_000 / rate
	if rhythm, ok := regularPeriod(sample.PeriodsUS); ok && rhythm < referencePeriodUS {
		referencePeriodUS = rhythm
	}
	gapDropouts := 0
	if gapTrusted && sample.MaxGapUS > 0 {
		gapDropouts = int(math.Max(0, math.Floor(float64(sample.MaxGapUS)/referencePeriodUS)-1))
	}
	minimumPulses := int(math.Floor(float64(windowMS) * 1000 / referencePeriodUS))
	observedPulses := int(sample.RisingEdges)
	if observedPulses >= minimumPulses {
		return gapDropouts
	}
	return maxInt(minimumPulses-observedPulses, gapDropouts)
}

// regularPeriod returns the median captured period when the pulse train is
// regular enough (coefficient of variation at most 20%) to define a rhythm.
func regularPeriod(periods []float64) (float64, bool) {
	if len(periods) < 3 {
		return 0, false
	}
	average := mean(periods)
	if average <= 0 || standardDeviation(periods)/average > 0.2 {
		return 0, false
	}
	sorted := append([]float64(nil), periods...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2], true
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
	if facts.CaptureUnreliable || facts.PulseWidthOutOfRange {
		return false
	}
	if facts.MissingExpectedActivity || facts.DropoutEvents > configuration.Expected.MaxDropouts {
		return false
	}
	if facts.FrequencyHz != nil && configuration.Expected.Required {
		if configuration.Expected.MinFrequencyHz != nil && *facts.FrequencyHz < *configuration.Expected.MinFrequencyHz ||
			configuration.Expected.MaxFrequencyHz != nil && *facts.FrequencyHz > *configuration.Expected.MaxFrequencyHz {
			return false
		}
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

// KnownGoodDeviations compares one window with a learned (multi-window)
// physical baseline envelope widened by its recorded tolerance. Baselines
// without a learned envelope are compared by baselineDeviation instead.
func KnownGoodDeviations(facts domain.DerivedFacts, baseline *domain.TrustedBaseline) []string {
	if baseline == nil || !baseline.Status.Trusted() || baseline.WindowCount == 0 {
		return nil
	}
	var deviations []string
	outside := func(value float64, low, high *float64, tolerancePct float64) bool {
		if low == nil || high == nil {
			return false
		}
		lowLimit := *low * (1 - tolerancePct/100)
		highLimit := *high * (1 + tolerancePct/100)
		return value < lowLimit || value > highLimit
	}
	if facts.AverageVoltage != nil && outside(*facts.AverageVoltage, baseline.MinVoltage, baseline.MaxVoltage, baseline.VoltageTolerancePct) {
		deviations = append(deviations, fmt.Sprintf("voltage %.3f V is outside the Known Good range %.3f–%.3f V (±%.0f%%)", *facts.AverageVoltage, *baseline.MinVoltage, *baseline.MaxVoltage, baseline.VoltageTolerancePct))
	}
	if baseline.MinFrequencyHz != nil {
		if facts.FrequencyHz == nil {
			deviations = append(deviations, "no measurable rate, but the Known Good capture had one")
		} else if outside(*facts.FrequencyHz, baseline.MinFrequencyHz, baseline.MaxFrequencyHz, baseline.FrequencyTolerancePct) {
			deviations = append(deviations, fmt.Sprintf("rate %.3f Hz is outside the Known Good range %.3f–%.3f Hz (±%.0f%%)", *facts.FrequencyHz, *baseline.MinFrequencyHz, *baseline.MaxFrequencyHz, baseline.FrequencyTolerancePct))
		}
	}
	if baseline.ComparePulseWidth && baseline.MinPulseWidthUS != nil && facts.AveragePulseWidthUS != nil &&
		outside(*facts.AveragePulseWidthUS, baseline.MinPulseWidthUS, baseline.MaxPulseWidthUS, baseline.PulseWidthTolerancePct) {
		deviations = append(deviations, fmt.Sprintf("pulse width %.1f µs is outside the Known Good range %.1f–%.1f µs (±%.0f%%)", *facts.AveragePulseWidthUS, *baseline.MinPulseWidthUS, *baseline.MaxPulseWidthUS, baseline.PulseWidthTolerancePct))
	}
	return deviations
}

func simultaneousGroups(probes []domain.DerivedFacts) [][]string {
	byBucket := make(map[int][]string)
	for _, facts := range probes {
		// An unreliable capture cannot corroborate a shared electrical cause.
		if facts.CaptureUnreliable {
			continue
		}
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
