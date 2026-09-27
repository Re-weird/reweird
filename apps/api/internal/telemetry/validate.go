package telemetry

import (
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/limits"
)

const SchemaVersion = 2

var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{2,63}$`)
var probePattern = regexp.MustCompile(`^P[1-6]$`)
var profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

func Validate(envelope domain.TelemetryEnvelope) error {
	if envelope.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %d", envelope.SchemaVersion)
	}
	if !deviceIDPattern.MatchString(envelope.DeviceID) {
		return errors.New("device_id must be 3-64 letters, digits, underscores, or hyphens")
	}
	if !profileIDPattern.MatchString(envelope.ProfileID) {
		return errors.New("profile_id must be 3-64 lowercase letters, digits, or hyphens")
	}
	if envelope.CapturedAtMS < 0 {
		return errors.New("captured_at_ms cannot be negative")
	}
	if envelope.WindowMS < 10 || envelope.WindowMS > 60_000 {
		return errors.New("window_ms must be between 10 and 60000")
	}
	if len(envelope.Samples) == 0 || len(envelope.Samples) > 6 {
		return errors.New("samples must contain 1-6 probe entries")
	}

	seen := make(map[string]bool, len(envelope.Samples))
	for index, sample := range envelope.Samples {
		if !probePattern.MatchString(sample.Probe) {
			return fmt.Errorf("samples[%d].probe must be P1-P6", index)
		}
		if seen[sample.Probe] {
			return fmt.Errorf("duplicate sample for %s", sample.Probe)
		}
		seen[sample.Probe] = true
		if err := validateSample(sample); err != nil {
			return fmt.Errorf("%s: %w", sample.Probe, err)
		}
	}
	return nil
}

func ValidateForProfile(envelope domain.TelemetryEnvelope, profile domain.ProjectProfile) error {
	if err := Validate(envelope); err != nil {
		return err
	}
	if !profile.Confirmed {
		return errors.New("project profile must be user-confirmed before hardware telemetry is accepted")
	}
	if envelope.ProfileID != profile.ID {
		return fmt.Errorf("Profile mismatch: firmware reports %s but this project expects %s (telemetry profile_id does not match active profile); reflash with the project's profile ID", envelope.ProfileID, profile.ID)
	}
	for _, sample := range envelope.Samples {
		configuration, ok := profile.Probe(sample.Probe)
		if !ok {
			return fmt.Errorf("%s is not assigned in project profile %s", sample.Probe, profile.ID)
		}
		if configuration.Mode != sample.Mode {
			return fmt.Errorf("%s mode mismatch: received %s, profile expects %s", sample.Probe, sample.Mode, configuration.Mode)
		}
		if configuration.SafeMeasurement.MaxPinVoltage <= 0 || configuration.SafeMeasurement.MaxPinVoltage > 3.3 {
			return fmt.Errorf("%s has invalid max_pin_voltage; ESP32 inputs must not exceed 3.3 V", sample.Probe)
		}
		if configuration.SafeMeasurement.InputScale <= 0 || configuration.SafeMeasurement.InputScale > 20 {
			return fmt.Errorf("%s input_scale must be greater than 0 and at most 20", sample.Probe)
		}
	}
	return nil
}

func validateSample(sample domain.TelemetrySample) error {
	if sample.Mode != domain.ProbeModeAnalog && sample.Mode != domain.ProbeModeDigital && sample.Mode != domain.ProbeModePulse {
		return fmt.Errorf("unsupported mode %q", sample.Mode)
	}
	analogLimit := limits.Bounded("MAX_TELEMETRY_ANALOG_SAMPLES", 256, 8, 256)
	pulseLimit := limits.Bounded("MAX_TELEMETRY_PULSE_SAMPLES", 512, 8, 512)
	if len(sample.AnalogMV) > analogLimit {
		return fmt.Errorf("analog_mv exceeds %d samples", analogLimit)
	}
	if len(sample.PeriodsUS) > pulseLimit || len(sample.HighPulseWidthsUS) > pulseLimit {
		return fmt.Errorf("pulse arrays exceed %d samples", pulseLimit)
	}
	if len(sample.ActivityCounts) > 120 {
		return errors.New("activity_counts exceeds 120 buckets")
	}
	if sample.State != nil && *sample.State != 0 && *sample.State != 1 {
		return errors.New("state must be 0 or 1")
	}
	if sample.RisingEdges+sample.FallingEdges > sample.EdgeCount {
		return errors.New("rising_edges + falling_edges cannot exceed edge_count")
	}
	if err := boundedFinite(sample.AnalogMV, 0, 3300, "analog_mv"); err != nil {
		return err
	}
	if err := boundedFinite(sample.PeriodsUS, 0, 60_000_000, "periods_us"); err != nil {
		return err
	}
	if err := boundedFinite(sample.HighPulseWidthsUS, 0, 60_000_000, "high_pulse_widths_us"); err != nil {
		return err
	}
	if err := boundedFinite(sample.ActivityCounts, 0, 10_000_000, "activity_counts"); err != nil {
		return err
	}
	return nil
}

func boundedFinite(values []float64, minimum, maximum float64, field string) error {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < minimum || value > maximum {
			return fmt.Errorf("%s contains an invalid value", field)
		}
	}
	return nil
}
