package passport

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
)

const defaultVoltageTolerancePct = 5.0
const defaultFrequencyTolerancePct = 10.0

func SourceKind(source string) (domain.BaselineSource, bool) {
	switch source {
	case "serial":
		return domain.BaselinePhysical, true
	case "simulator":
		return domain.BaselineSimulated, true
	default:
		return "", false
	}
}

func CaptureSummary(window domain.MeasurementWindow) (domain.PassportCapture, bool) {
	source, ok := SourceKind(window.Source)
	if !ok {
		return domain.PassportCapture{}, false
	}
	return domain.PassportCapture{MeasurementID: window.ID, Source: source, DeviceID: window.DeviceID, CapturedAtMS: window.CapturedAtMS, IngestedAtMS: window.IngestedAtMS, WindowMS: window.Analysis.WindowMS, Probes: window.Analysis.Probes}, true
}

func BuildKnownGood(profile domain.ProjectProfile, window domain.MeasurementWindow, note string) (domain.KnownGoodBaseline, error) {
	if !profile.Confirmed || profile.ID == "" {
		return domain.KnownGoodBaseline{}, errors.New("a confirmed Project Profile is required")
	}
	source, ok := SourceKind(window.Source)
	if !ok {
		return domain.KnownGoodBaseline{}, errors.New("only stored serial or simulator captures can become a baseline")
	}
	if window.ID <= 0 || window.ProfileID != profile.ID || window.Raw.ProfileID != profile.ID || window.Analysis.ProfileID != profile.ID ||
		window.DeviceID == "" || window.Raw.DeviceID != window.DeviceID || window.Analysis.DeviceID != window.DeviceID ||
		window.IngestedAtMS <= 0 || window.CapturedAtMS < 0 || window.Raw.CapturedAtMS != window.CapturedAtMS || window.Analysis.CapturedAtMS != window.CapturedAtMS ||
		window.Raw.SchemaVersion != 2 || window.Analysis.SchemaVersion != 2 || window.Raw.WindowMS == 0 || window.Analysis.WindowMS != window.Raw.WindowMS {
		return domain.KnownGoodBaseline{}, errors.New("stored capture does not match the confirmed profile or its raw and derived evidence")
	}
	if err := telemetry.ValidateForProfile(window.Raw, profile); err != nil {
		return domain.KnownGoodBaseline{}, fmt.Errorf("stored raw capture is invalid: %w", err)
	}
	status := domain.BaselineKnownGoodCapture
	if source == domain.BaselinePhysical {
		status = domain.BaselineUserConfirmedHealthy
	}
	record := domain.KnownGoodBaseline{
		ProfileID: profile.ID, ProfileVersion: profile.Version, MeasurementID: window.ID,
		Source: source, DeviceID: window.DeviceID, CapturedAtMS: window.CapturedAtMS, IngestedAtMS: window.IngestedAtMS,
		SavedAtMS: time.Now().UTC().UnixMilli(), ConfirmedBy: "local_user_reported", Note: strings.TrimSpace(note),
		Probes: make([]domain.BaselineProbe, 0, len(profile.Probes)),
	}
	for _, configuration := range profile.Probes {
		if !configuration.Expected.Required || strings.EqualFold(configuration.Role, "UNASSIGNED") {
			continue
		}
		rawPresent := false
		for _, sample := range window.Raw.Samples {
			if sample.Probe == configuration.Probe {
				rawPresent = true
				break
			}
		}
		if !rawPresent {
			return domain.KnownGoodBaseline{}, fmt.Errorf("%s has no raw sample in the stored capture", configuration.Probe)
		}
		facts, present := window.Analysis.Probe(configuration.Probe)
		if !present || facts.Mode != configuration.Mode || !facts.Stable || facts.MissingExpectedActivity || facts.DropoutEvents > configuration.Expected.MaxDropouts {
			return domain.KnownGoodBaseline{}, fmt.Errorf("%s is missing or outside its expected stable behavior", configuration.Probe)
		}
		voltageTolerance := configuration.Expected.VoltageTolerancePct
		if voltageTolerance <= 0 {
			voltageTolerance = defaultVoltageTolerancePct
		}
		if configuration.Expected.MinVoltage != nil || configuration.Expected.MaxVoltage != nil || configuration.Expected.NominalVoltage != nil {
			if facts.AverageVoltage == nil {
				return domain.KnownGoodBaseline{}, fmt.Errorf("%s has no measured voltage for its configured expectation", configuration.Probe)
			}
			voltage := *facts.AverageVoltage
			if configuration.Expected.MinVoltage != nil && voltage < *configuration.Expected.MinVoltage ||
				configuration.Expected.MaxVoltage != nil && voltage > *configuration.Expected.MaxVoltage ||
				configuration.Expected.NominalVoltage != nil && *configuration.Expected.NominalVoltage > 0 &&
					percentDifference(voltage, *configuration.Expected.NominalVoltage) > voltageTolerance {
				return domain.KnownGoodBaseline{}, fmt.Errorf("%s voltage is outside its configured expectation", configuration.Probe)
			}
		}
		if (configuration.Expected.MinFrequencyHz != nil || configuration.Expected.MaxFrequencyHz != nil || configuration.Expected.NominalFrequencyHz != nil) && facts.FrequencyHz == nil {
			return domain.KnownGoodBaseline{}, fmt.Errorf("%s has no measured frequency for its configured expectation", configuration.Probe)
		}
		if facts.FrequencyHz != nil {
			frequency := *facts.FrequencyHz
			if configuration.Expected.MinFrequencyHz != nil && frequency < *configuration.Expected.MinFrequencyHz ||
				configuration.Expected.MaxFrequencyHz != nil && frequency > *configuration.Expected.MaxFrequencyHz ||
				configuration.Expected.NominalFrequencyHz != nil && *configuration.Expected.NominalFrequencyHz > 0 &&
					percentDifference(frequency, *configuration.Expected.NominalFrequencyHz) > defaultFrequencyTolerancePct {
				return domain.KnownGoodBaseline{}, fmt.Errorf("%s frequency is outside its configured expectation", configuration.Probe)
			}
		}
		trusted := domain.TrustedBaseline{
			Status: status, CapturedAtMS: window.CapturedAtMS,
			AverageVoltage: facts.AverageVoltage, FrequencyHz: facts.FrequencyHz,
			DropoutsPerWindow: &facts.DropoutEvents, VoltageVariation: facts.VoltageVariation,
			VoltageTolerancePct: voltageTolerance, FrequencyTolerancePct: defaultFrequencyTolerancePct,
		}
		record.Probes = append(record.Probes, domain.BaselineProbe{Probe: facts.Probe, Role: configuration.Role, Facts: facts, Trusted: trusted})
	}
	if len(record.Probes) == 0 {
		return domain.KnownGoodBaseline{}, errors.New("no required configured probe has a comparable capture")
	}
	return record, nil
}

func ApplyKnownGood(profile domain.ProjectProfile, source domain.BaselineSource, baseline *domain.KnownGoodBaseline) domain.ProjectProfile {
	copyOfProfile := profile
	copyOfProfile.Probes = append([]domain.ProbeConfiguration(nil), profile.Probes...)
	for index := range copyOfProfile.Probes {
		configuration := &copyOfProfile.Probes[index]
		// Seeded demo/user-provided capture values are never a physical baseline.
		if source == domain.BaselinePhysical && configuration.Baseline != nil && configuration.Baseline.Status != domain.BaselineManufacturerSpec {
			configuration.Baseline = nil
		}
		if baseline == nil || baseline.ProfileID != profile.ID || baseline.ProfileVersion != profile.Version || baseline.Source != source {
			continue
		}
		for _, probe := range baseline.Probes {
			if probe.Probe == configuration.Probe {
				trusted := probe.Trusted
				configuration.Baseline = &trusted
				break
			}
		}
	}
	return copyOfProfile
}

func CurrentStatus(profile domain.ProjectProfile, window *domain.MeasurementWindow, baseline *domain.KnownGoodBaseline, hasPhysicalBaseline bool) (domain.PassportStatus, string) {
	if window == nil {
		if hasPhysicalBaseline {
			return domain.PassportNeedsVerification, "A physical baseline exists, but no current capture is available."
		}
		return domain.PassportNoPhysicalBaseline, "No user-confirmed physical baseline has been saved."
	}
	source, valid := SourceKind(window.Source)
	if !valid {
		return domain.PassportNeedsVerification, "The latest capture is not from a supported telemetry source."
	}
	if window.ProfileID != profile.ID || window.Raw.ProfileID != window.ProfileID || window.Analysis.ProfileID != window.ProfileID ||
		window.DeviceID == "" || window.Raw.DeviceID != window.DeviceID || window.Analysis.DeviceID != window.DeviceID {
		return domain.PassportNeedsVerification, "The latest capture has inconsistent profile or device identity."
	}
	if baseline != nil && baseline.ProfileVersion != profile.Version {
		return domain.PassportNeedsVerification, "The Project Profile changed since this baseline was saved; capture and confirm a new reference."
	}
	if baseline == nil || baseline.Source != source || baseline.DeviceID != window.DeviceID || baseline.ProfileID != window.ProfileID {
		if source == domain.BaselineSimulated {
			if hasPhysicalBaseline {
				return domain.PassportNeedsVerification, "Latest capture is simulated; it cannot verify the saved physical baseline."
			}
			return domain.PassportNoPhysicalBaseline, "Latest capture is simulated; no matching simulated baseline is saved. This is not physical health evidence."
		}
		return domain.PassportNoPhysicalBaseline, "No user-confirmed physical baseline matches this device."
	}
	if window.ID <= baseline.MeasurementID {
		if source == domain.BaselineSimulated {
			return domain.PassportSimulatedBaseline, "Simulated Known Good saved. Capture another scenario to compare; no physical health is claimed."
		}
		return domain.PassportNeedsVerification, "Physical Known Good saved. Capture a later window to verify current behavior."
	}
	issues := make([]string, 0)
	for _, probe := range baseline.Probes {
		facts, ok := window.Analysis.Probe(probe.Probe)
		if !ok {
			issues = append(issues, probe.Probe+" missing from current capture")
			continue
		}
		if !facts.Stable || facts.MissingExpectedActivity || facts.DropoutEvents > probe.Facts.DropoutEvents {
			issues = append(issues, probe.Probe+" activity or stability changed")
		}
		configuration, configured := profile.Probe(probe.Probe)
		if !configured || configuration.Mode != facts.Mode {
			issues = append(issues, probe.Probe+" no longer matches the confirmed probe configuration")
			continue
		}
		if facts.AverageVoltage != nil && (configuration.Expected.MinVoltage != nil && *facts.AverageVoltage < *configuration.Expected.MinVoltage ||
			configuration.Expected.MaxVoltage != nil && *facts.AverageVoltage > *configuration.Expected.MaxVoltage) {
			issues = append(issues, probe.Probe+" voltage is outside profile limits")
		}
		if facts.FrequencyHz != nil && (configuration.Expected.MinFrequencyHz != nil && *facts.FrequencyHz < *configuration.Expected.MinFrequencyHz ||
			configuration.Expected.MaxFrequencyHz != nil && *facts.FrequencyHz > *configuration.Expected.MaxFrequencyHz) {
			issues = append(issues, probe.Probe+" frequency is outside profile limits")
		}
		if probe.Trusted.AverageVoltage != nil && facts.AverageVoltage == nil || probe.Trusted.FrequencyHz != nil && facts.FrequencyHz == nil {
			issues = append(issues, probe.Probe+" comparable measurement is missing")
		}
		if facts.AverageVoltage != nil && probe.Trusted.AverageVoltage != nil &&
			percentDifference(*facts.AverageVoltage, *probe.Trusted.AverageVoltage) > probe.Trusted.VoltageTolerancePct {
			issues = append(issues, probe.Probe+" voltage deviated")
		}
		if facts.FrequencyHz != nil && probe.Trusted.FrequencyHz != nil &&
			percentDifference(*facts.FrequencyHz, *probe.Trusted.FrequencyHz) > probe.Trusted.FrequencyTolerancePct {
			issues = append(issues, probe.Probe+" frequency deviated")
		}
	}
	if len(issues) > 0 {
		if source == domain.BaselineSimulated {
			return domain.PassportSimulatedDeviation, "Simulated deviation: " + strings.Join(issues, "; ")
		}
		return domain.PassportDeviation, "Physical deviation: " + strings.Join(issues, "; ")
	}
	if source == domain.BaselineSimulated {
		return domain.PassportSimulatedMatch, "Simulated capture matches its simulated baseline. No physical health is claimed."
	}
	return domain.PassportHealthy, "Latest physical capture matches the user-confirmed physical baseline within configured tolerances."
}

func percentDifference(value, reference float64) float64 {
	if reference == 0 {
		if value == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return math.Abs(value-reference) / math.Abs(reference) * 100
}
