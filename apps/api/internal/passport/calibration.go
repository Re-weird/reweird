package passport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
)

// CalibrationWindows is how many consecutive physical windows must be
// observed before ReWeird asks the user to confirm the circuit is healthy.
const CalibrationWindows = 10

// Margins added around the learned envelope. They are fixed and documented,
// not tuned per circuit: the envelope itself comes from the observation.
const (
	learnedVoltageMarginPct    = 2.0
	learnedFrequencyMarginPct  = 5.0
	learnedPulseWidthMarginPct = 10.0
)

// ProbeMapping returns the confirmed probe configuration in a stable order.
func ProbeMapping(profile domain.ProjectProfile) []domain.ProbeMappingEntry {
	mapping := make([]domain.ProbeMappingEntry, 0, len(profile.Probes))
	for _, probe := range profile.Probes {
		mapping = append(mapping, domain.ProbeMappingEntry{
			Probe: probe.Probe, Role: probe.Role, Mode: probe.Mode,
			InputScale: probe.SafeMeasurement.InputScale, Required: probe.Expected.Required,
		})
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].Probe < mapping[j].Probe })
	return mapping
}

// MappingHash fingerprints the probe mapping so a baseline learned under one
// wiring can never silently validate another.
func MappingHash(profile domain.ProjectProfile) string {
	var builder strings.Builder
	for _, entry := range ProbeMapping(profile) {
		fmt.Fprintf(&builder, "%s|%s|%s|%g|%t;", entry.Probe, strings.ToUpper(entry.Role), entry.Mode, entry.InputScale, entry.Required)
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:8])
}

// BaselineCompatible reports whether a baseline was learned under exactly
// this profile revision and probe mapping.
func BaselineCompatible(profile domain.ProjectProfile, baseline *domain.KnownGoodBaseline) (bool, string) {
	if baseline == nil {
		return false, "no baseline"
	}
	if baseline.ProfileID != profile.ID {
		return false, "the baseline belongs to a different profile"
	}
	if baseline.ProfileVersion != profile.Version {
		return false, fmt.Sprintf("the baseline was learned under profile revision %d; the active revision is %d", baseline.ProfileVersion, profile.Version)
	}
	if baseline.ProbeMappingHash != "" && baseline.ProbeMappingHash != MappingHash(profile) {
		return false, "the probe mapping changed since the baseline was learned"
	}
	return true, ""
}

// ConsecutiveRun returns the newest uninterrupted run of physical windows
// for one device and profile revision, oldest first. windows may be in any
// order. A gap in the device sequence ends the run.
func ConsecutiveRun(profile domain.ProjectProfile, windows []domain.MeasurementWindow, deviceID string, endingAt int64, limit int) []domain.MeasurementWindow {
	candidates := make([]domain.MeasurementWindow, 0, len(windows))
	for _, window := range windows {
		if window.Source != "serial" || window.ProfileID != profile.ID || window.Analysis.ProfileVersion != profile.Version {
			continue
		}
		if deviceID != "" && window.DeviceID != deviceID {
			continue
		}
		if endingAt > 0 && window.ID > endingAt {
			continue
		}
		candidates = append(candidates, window)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID > candidates[j].ID })
	run := make([]domain.MeasurementWindow, 0, limit)
	for _, window := range candidates {
		if len(run) > 0 {
			previous := run[len(run)-1]
			if window.DeviceID != previous.DeviceID || window.Sequence+1 != previous.Sequence {
				break
			}
		}
		run = append(run, window)
		if len(run) == limit {
			break
		}
	}
	for left, right := 0, len(run)-1; left < right; left, right = left+1, right-1 {
		run[left], run[right] = run[right], run[left]
	}
	return run
}

// LearnKnownGood turns a confirmed run of physical windows into a Known Good
// baseline. It refuses simulated input, mixed devices, other profile
// revisions, too few windows, and any window that violates the configured
// expectations or has an unreliable capture. Confirmation that the circuit
// is healthy is the caller's responsibility and must be explicit.
func LearnKnownGood(profile domain.ProjectProfile, run []domain.MeasurementWindow, note string) (domain.KnownGoodBaseline, error) {
	if len(run) < CalibrationWindows {
		return domain.KnownGoodBaseline{}, fmt.Errorf("observe at least %d consecutive physical windows before saving Known Good (have %d)", CalibrationWindows, len(run))
	}
	deviceID := run[0].DeviceID
	for index, window := range run {
		if window.Source != "serial" {
			return domain.KnownGoodBaseline{}, errors.New("simulated or non-serial captures can never establish a physical Known Good")
		}
		if window.DeviceID != deviceID {
			return domain.KnownGoodBaseline{}, errors.New("the observed windows come from more than one device")
		}
		if window.Analysis.ProfileVersion != profile.Version {
			return domain.KnownGoodBaseline{}, fmt.Errorf("capture #%d was analyzed under profile revision %d, not the active revision %d", window.ID, window.Analysis.ProfileVersion, profile.Version)
		}
		if index > 0 && window.Sequence != run[index-1].Sequence+1 {
			return domain.KnownGoodBaseline{}, fmt.Errorf("the observation is interrupted between captures #%d and #%d", run[index-1].ID, window.ID)
		}
	}
	perWindow := make([]domain.KnownGoodBaseline, 0, len(run))
	for _, window := range run {
		record, err := BuildKnownGood(profile, window, note)
		if err != nil {
			return domain.KnownGoodBaseline{}, fmt.Errorf("capture #%d: %w", window.ID, err)
		}
		perWindow = append(perWindow, record)
	}
	latest := perWindow[len(perWindow)-1]
	learned := latest
	learned.Source = domain.BaselinePhysical
	learned.Provenance = domain.ProvenanceRealSerial
	learned.ProbeMapping = ProbeMapping(profile)
	learned.ProbeMappingHash = MappingHash(profile)
	learned.WindowCount = len(run)
	learned.FirstMeasurementID = run[0].ID
	learned.LastMeasurementID = run[len(run)-1].ID
	learned.Probes = make([]domain.BaselineProbe, 0, len(latest.Probes))
	for _, latestProbe := range latest.Probes {
		configuration, _ := profile.Probe(latestProbe.Probe)
		facts := make([]domain.DerivedFacts, 0, len(run))
		for _, window := range run {
			if fact, ok := window.Analysis.Probe(latestProbe.Probe); ok {
				facts = append(facts, fact)
			}
		}
		trusted := learnEnvelope(facts, configuration)
		trusted.CapturedAtMS = latestProbe.Trusted.CapturedAtMS
		learned.Probes = append(learned.Probes, domain.BaselineProbe{Probe: latestProbe.Probe, Role: latestProbe.Role, Facts: latestProbe.Facts, Trusted: trusted})
	}
	return learned, nil
}

func learnEnvelope(facts []domain.DerivedFacts, configuration domain.ProbeConfiguration) domain.TrustedBaseline {
	summary := Summarize(facts)
	voltageMargin := learnedVoltageMarginPct
	maxDropouts := summary.MaxDropouts
	trusted := domain.TrustedBaseline{
		Status: domain.BaselineUserConfirmedHealthy, WindowCount: summary.Windows,
		AverageVoltage: summary.AverageVoltage, MinVoltage: summary.MinVoltage, MaxVoltage: summary.MaxVoltage,
		VoltageVariation: summary.MaxVoltageVariation, VoltageTolerancePct: voltageMargin,
		FrequencyHz: summary.AverageFrequencyHz, MinFrequencyHz: summary.MinFrequencyHz, MaxFrequencyHz: summary.MaxFrequencyHz,
		FrequencyTolerancePct: learnedFrequencyMarginPct,
		PulseWidthUS:          summary.AveragePulseWidthUS, MinPulseWidthUS: summary.MinPulseWidthUS, MaxPulseWidthUS: summary.MaxPulseWidthUS,
		PulseWidthTolerancePct: learnedPulseWidthMarginPct,
		ComparePulseWidth:      configuration.Expected.Stable || configuration.Expected.MinPulseWidthUS != nil || configuration.Expected.MaxPulseWidthUS != nil,
		DropoutsPerWindow:      &maxDropouts,
	}
	return trusted
}

// Summarize aggregates derived facts from several windows of one probe.
func Summarize(facts []domain.DerivedFacts) domain.ObservedSummary {
	summary := domain.ObservedSummary{Windows: len(facts)}
	var voltages, frequencies, widths []float64
	seenIssues := map[string]bool{}
	for _, fact := range facts {
		if fact.Stable {
			summary.StableWindows++
		}
		if fact.CaptureUnreliable {
			summary.UnreliableWindows++
			for _, issue := range fact.CaptureIssues {
				if !seenIssues[issue] && len(summary.CaptureIssues) < 5 {
					seenIssues[issue] = true
					summary.CaptureIssues = append(summary.CaptureIssues, issue)
				}
			}
		}
		if fact.DigitalTransitions > 0 || fact.PulseCount > 0 || fact.AverageVoltage != nil && *fact.AverageVoltage > 0.01 {
			summary.ActiveWindows++
		}
		if fact.DropoutEvents > summary.MaxDropouts {
			summary.MaxDropouts = fact.DropoutEvents
		}
		if fact.AverageVoltage != nil {
			voltages = append(voltages, *fact.AverageVoltage)
			summary.MinVoltage = minPointer(summary.MinVoltage, fact.MinimumVoltage, fact.AverageVoltage)
			summary.MaxVoltage = maxPointer(summary.MaxVoltage, fact.MaximumVoltage, fact.AverageVoltage)
			summary.MaxVoltageVariation = maxPointer(summary.MaxVoltageVariation, fact.VoltageVariation)
		}
		if fact.FrequencyHz != nil {
			frequencies = append(frequencies, *fact.FrequencyHz)
		}
		if fact.AveragePulseWidthUS != nil {
			widths = append(widths, *fact.AveragePulseWidthUS)
			summary.MinPulseWidthUS = minPointer(summary.MinPulseWidthUS, fact.MinimumPulseWidthUS, fact.AveragePulseWidthUS)
			summary.MaxPulseWidthUS = maxPointer(summary.MaxPulseWidthUS, fact.MaximumPulseWidthUS, fact.AveragePulseWidthUS)
		}
	}
	if len(voltages) > 0 {
		summary.AverageVoltage = pointer(average(voltages))
	}
	if len(frequencies) > 0 {
		low, high := frequencies[0], frequencies[0]
		for _, value := range frequencies {
			low, high = math.Min(low, value), math.Max(high, value)
		}
		summary.MinFrequencyHz, summary.MaxFrequencyHz = pointer(low), pointer(high)
		summary.AverageFrequencyHz = pointer(average(frequencies))
	}
	if len(widths) > 0 {
		summary.AveragePulseWidthUS = pointer(average(widths))
	}
	return summary
}

// CalibrationInput is everything ComputeCalibration needs; the handler
// gathers it from storage so the state machine stays pure and testable.
type CalibrationInput struct {
	Profile         domain.ProjectProfile
	ActiveSource    string // "serial" or "simulator"
	Windows         []domain.MeasurementWindow
	KnownGood       *domain.KnownGoodBaseline // latest physical baseline for the observed device
	PhysicalAllowed bool
	PhysicalBlock   string // why physical calibration is not allowed for this profile
	NotBeforeMS     int64  // captures ingested before probe confirmation are ignored
}

func ComputeCalibration(input CalibrationInput) domain.CalibrationState {
	profile := input.Profile
	state := domain.CalibrationState{
		ProfileID: profile.ID, ProfileVersion: profile.Version, WindowsRequired: CalibrationWindows,
		ProbeMappingHash: MappingHash(profile), Probes: make([]domain.CalibrationProbe, 0, len(profile.Probes)),
	}
	eligible := make([]domain.MeasurementWindow, 0, len(input.Windows))
	for _, window := range input.Windows {
		if window.IngestedAtMS >= input.NotBeforeMS {
			eligible = append(eligible, window)
		}
	}
	deviceID := ""
	newestID := int64(0)
	for _, window := range eligible {
		if window.Source == "serial" && window.ProfileID == profile.ID && window.ID > newestID {
			newestID, deviceID = window.ID, window.DeviceID
		}
	}
	run := ConsecutiveRun(profile, eligible, deviceID, 0, CalibrationWindows)
	state.DeviceID = deviceID
	state.WindowsObserved = len(run)
	if len(run) > 0 {
		state.Provenance = domain.ProvenanceRealSerial
		state.FirstMeasurementID = run[0].ID
		state.CandidateMeasurementID = run[len(run)-1].ID
	}

	var baselineProbes map[string]domain.TrustedBaseline
	compatible := false
	if input.KnownGood != nil && input.KnownGood.Source == domain.BaselinePhysical {
		state.KnownGood = input.KnownGood
		var reason string
		compatible, reason = BaselineCompatible(profile, input.KnownGood)
		if compatible {
			baselineProbes = map[string]domain.TrustedBaseline{}
			for _, probe := range input.KnownGood.Probes {
				baselineProbes[probe.Probe] = probe.Trusted
			}
		} else {
			state.Blockers = append(state.Blockers, "Existing Known Good is incompatible: "+reason+".")
		}
	}

	for _, configuration := range profile.Probes {
		facts := make([]domain.DerivedFacts, 0, len(run))
		for _, window := range run {
			if fact, ok := window.Analysis.Probe(configuration.Probe); ok {
				facts = append(facts, fact)
			}
		}
		probe := domain.CalibrationProbe{
			Probe: configuration.Probe, Role: configuration.Role, Mode: configuration.Mode,
			Required: configuration.Expected.Required, Expected: configuration.Expected, Observed: Summarize(facts),
		}
		if trusted, ok := baselineProbes[configuration.Probe]; ok {
			copyOfTrusted := trusted
			probe.KnownGood = &copyOfTrusted
		}
		if len(facts) > 0 {
			latest := facts[len(facts)-1]
			unassigned := strings.EqualFold(configuration.Role, "UNASSIGNED")
			switch {
			case unassigned && probe.Observed.ActiveWindows > 0:
				probe.Issues = append(probe.Issues, "Activity observed on an unassigned probe. Assign its role in the profile, or confirm it is intentionally ignored.")
			case !unassigned:
				probe.Issues = append(probe.Issues, latest.CaptureIssues...)
				if latest.PulseWidthOutOfRange {
					probe.Issues = append(probe.Issues, "Pulse width is outside the configured expectation.")
				}
				if latest.MissingExpectedActivity {
					probe.Issues = append(probe.Issues, "No expected activity in the latest window.")
				}
				if configuration.Expected.Required && latest.DropoutEvents > configuration.Expected.MaxDropouts {
					probe.Issues = append(probe.Issues, fmt.Sprintf("%d dropouts in the latest window (configured maximum %d).", latest.DropoutEvents, configuration.Expected.MaxDropouts))
				}
				if probe.KnownGood != nil {
					probe.Issues = append(probe.Issues, signalanalysis.KnownGoodDeviations(latest, probe.KnownGood)...)
				}
			}
		}
		state.Probes = append(state.Probes, probe)
	}

	switch {
	case input.ActiveSource == "simulator":
		state.Status = domain.CalibrationSimulatedSource
		state.Detail = "The active telemetry is simulated. Simulated captures can never establish or verify a physical Known Good."
		return state
	case !input.PhysicalAllowed:
		state.Status = domain.CalibrationNotCalibrated
		state.Detail = input.PhysicalBlock
		state.Blockers = append(state.Blockers, input.PhysicalBlock)
		return state
	case compatible:
		state.Status = domain.CalibrationCalibrated
		state.Detail = fmt.Sprintf("Known Good learned from %d REAL SERIAL windows of %s under profile revision %d.", input.KnownGood.WindowCount, input.KnownGood.DeviceID, input.KnownGood.ProfileVersion)
		return state
	}

	incompatible := state.KnownGood != nil
	switch {
	case len(run) == 0:
		state.Status = domain.CalibrationNotCalibrated
		state.Detail = "No physical capture for this profile revision yet. Connect the ReWeird device and start serial telemetry."
	case len(run) < CalibrationWindows:
		state.Status = domain.CalibrationObserving
		state.Detail = fmt.Sprintf("Observing physical behavior: %d of %d consecutive windows.", len(run), CalibrationWindows)
	default:
		if _, err := LearnKnownGood(profile, run, ""); err != nil {
			state.Status = domain.CalibrationAttentionRequired
			state.Detail = "The observed behavior cannot be saved as Known Good."
			state.Blockers = append(state.Blockers, err.Error())
		} else {
			state.Status = domain.CalibrationReviewRequired
			state.Detail = fmt.Sprintf("%d consecutive windows are stable and within the configured expectations. Confirm the circuit is operating correctly to save them as Known Good.", len(run))
			state.CanSaveKnownGood = true
		}
	}
	if incompatible && state.Status != domain.CalibrationReviewRequired {
		state.Status = domain.CalibrationBaselineIncompatible
		state.Detail = "The saved Known Good does not match this profile revision or probe mapping. " + state.Detail
	}
	return state
}

func pointer(value float64) *float64 { return &value }

func average(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func minPointer(current *float64, candidates ...*float64) *float64 {
	for _, candidate := range candidates {
		if candidate != nil {
			if current == nil || *candidate < *current {
				current = pointer(*candidate)
			}
			break
		}
	}
	return current
}

func maxPointer(current *float64, candidates ...*float64) *float64 {
	for _, candidate := range candidates {
		if candidate != nil {
			if current == nil || *candidate > *current {
				current = pointer(*candidate)
			}
			break
		}
	}
	return current
}
