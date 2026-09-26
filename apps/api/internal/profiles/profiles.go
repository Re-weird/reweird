package profiles

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

func UltrasonicDemo() domain.ProjectProfile {
	now := time.Now().UTC().UnixMilli()
	return domain.ProjectProfile{
		ID:               "ultrasonic-demo",
		ProjectName:      "Ultrasonic Distance Sensor",
		Controller:       "ESP32",
		LogicVoltage:     3.3,
		Confirmed:        true,
		ExpectedBehavior: "Measure distance continuously using an HC-SR04 trigger and echo pulse pair.",
		CreatedAtMS:      now,
		UpdatedAtMS:      now,
		Components: []domain.ComponentSpecification{{
			ID:           "hc-sr04",
			Name:         "HC-SR04 Ultrasonic Distance Sensor",
			Manufacturer: "Generic",
			Properties:   map[string]float64{"supply_voltage": 5, "logic_voltage": 3.3},
			Source:       "component-catalog/hc-sr04",
		}},
		Probes: []domain.ProbeConfiguration{
			{
				Probe:           "P1",
				Role:            "POWER",
				Mode:            domain.ProbeModeAnalog,
				IsPowerRail:     true,
				Expected:        domain.ExpectedSignal{SignalType: "voltage rail", Required: true, Stable: true, MinVoltage: number(4.75), MaxVoltage: number(5.25), NominalVoltage: number(5), VoltageTolerancePct: 2, MaxDropouts: 0},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 2, Notes: "Requires a verified 2:1 divider before the ESP32 ADC input."},
				Baseline:        &domain.TrustedBaseline{Status: domain.BaselineKnownGoodCapture, AverageVoltage: number(5.01), VoltageVariation: number(0.02), VoltageTolerancePct: 2},
			},
			{
				Probe:           "P2",
				Role:            "TRIG",
				Mode:            domain.ProbeModePulse,
				Expected:        domain.ExpectedSignal{SignalType: "pulse", Required: true, Stable: true, MinFrequencyHz: number(39_000), MaxFrequencyHz: number(41_000), NominalFrequencyHz: number(40_000), MaxDropouts: 0},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1, Notes: "Input-only monitor; never drives the target TRIG line."},
				Baseline:        &domain.TrustedBaseline{Status: domain.BaselineKnownGoodCapture, FrequencyHz: number(40_000), FrequencyTolerancePct: 3},
			},
			{
				Probe:           "P3",
				Role:            "ECHO",
				Mode:            domain.ProbeModePulse,
				Expected:        domain.ExpectedSignal{SignalType: "return pulse", Required: true, Stable: true, MinFrequencyHz: number(1), MaxFrequencyHz: number(100), NominalFrequencyHz: number(28.4), MaxDropouts: 0},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1, Notes: "HC-SR04 ECHO requires a level shifter or divider; never connect a 5 V ECHO directly."},
				Baseline:        &domain.TrustedBaseline{Status: domain.BaselineUserConfirmedHealthy, FrequencyHz: number(28.4), DropoutsPerWindow: integer(0), FrequencyTolerancePct: 20},
			},
			unassigned("P4"),
			unassigned("P5"),
			unassigned("P6"),
		},
	}
}

func Validate(profile domain.ProjectProfile) error {
	if !profileIDPattern.MatchString(profile.ID) {
		return errors.New("profile id must be 3-64 lowercase letters, digits, or hyphens")
	}
	if profile.ProjectName == "" || len(profile.ProjectName) > 120 {
		return errors.New("project_name is required and must not exceed 120 characters")
	}
	if profile.Controller == "" || len(profile.Controller) > 80 {
		return errors.New("controller is required and must not exceed 80 characters")
	}
	if profile.LogicVoltage <= 0 || profile.LogicVoltage > 5.5 {
		return errors.New("logic_voltage must be greater than 0 and at most 5.5 V")
	}
	if len(profile.Probes) == 0 || len(profile.Probes) > 6 {
		return errors.New("profile must contain 1-6 probe configurations")
	}
	seen := make(map[string]bool, len(profile.Probes))
	for _, probe := range profile.Probes {
		if !regexp.MustCompile(`^P[1-6]$`).MatchString(probe.Probe) {
			return fmt.Errorf("invalid probe %q", probe.Probe)
		}
		if seen[probe.Probe] {
			return fmt.Errorf("duplicate probe %s", probe.Probe)
		}
		seen[probe.Probe] = true
		if probe.Role == "" || len(probe.Role) > 40 {
			return fmt.Errorf("%s role is required and must not exceed 40 characters", probe.Probe)
		}
		if probe.Mode != domain.ProbeModeAnalog && probe.Mode != domain.ProbeModeDigital && probe.Mode != domain.ProbeModePulse {
			return fmt.Errorf("%s has unsupported mode %q", probe.Probe, probe.Mode)
		}
		if probe.SafeMeasurement.MaxPinVoltage <= 0 || probe.SafeMeasurement.MaxPinVoltage > 3.3 {
			return fmt.Errorf("%s max_pin_voltage must be greater than 0 and at most 3.3 V", probe.Probe)
		}
		if probe.SafeMeasurement.InputScale <= 0 || probe.SafeMeasurement.InputScale > 20 {
			return fmt.Errorf("%s input_scale must be greater than 0 and at most 20", probe.Probe)
		}
	}
	return nil
}

func unassigned(probe string) domain.ProbeConfiguration {
	return domain.ProbeConfiguration{
		Probe:           probe,
		Role:            "UNASSIGNED",
		Mode:            domain.ProbeModeDigital,
		Expected:        domain.ExpectedSignal{SignalType: "unassigned", Required: false},
		SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1, Notes: "Passive input only."},
	}
}

func number(value float64) *float64 { return &value }

func integer(value int) *int { return &value }
