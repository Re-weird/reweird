package projectunderstanding

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func GenerateProbePlan(profile domain.ProjectProfile) (domain.ProbePlan, []domain.ProbeConfiguration, error) {
	if len(profile.Connections) == 0 {
		return domain.ProbePlan{}, nil, errors.New("no measurable profile connections are available for probe mapping")
	}
	now := time.Now().UTC().UnixMilli()
	plan := domain.ProbePlan{ProjectID: profile.ProjectID, ProfileID: profile.ID, GeneratedAtMS: now, Instructions: []domain.ProbeInstruction{{Probe: "GND", Role: "reference", Target: "Target circuit ground", Expected: "common reference", SignalType: "ground", SafeWarning: "Connect ReWeird GND only to the target circuit ground after power and polarity are verified.", Explanation: "Every passive measurement needs the same reference as the target circuit."}}}
	configurations := make([]domain.ProbeConfiguration, 0, 6)
	probeNumber := 1
	for _, connection := range profile.Connections {
		if probeNumber > 6 || strings.EqualFold(connection.Role, "GND") || strings.Contains(strings.ToLower(connection.Behavior), "ground") {
			continue
		}
		probe := fmt.Sprintf("P%d", probeNumber)
		mode := probeMode(connection.Behavior)
		scale := 1.0
		warning := "Passive input only. Verify the node never exceeds 3.3 V at the ESP32 input."
		if connection.Expected.MaxVoltage != nil && *connection.Expected.MaxVoltage > 3.3 || connection.Expected.NominalVoltage != nil && *connection.Expected.NominalVoltage > 3.3 {
			scale = 2
			warning = "This node may exceed 3.3 V. Use a verified divider or level shifter before connecting the ReWeird input."
		}
		plan.Instructions = append(plan.Instructions, domain.ProbeInstruction{Probe: probe, Role: connection.Role, Target: connection.Target, Expected: expectedDescription(connection.Expected), SignalType: connection.Expected.SignalType, SafeWarning: warning, Explanation: "Generated from the confirmed " + connection.ComponentName + " connection."})
		configurations = append(configurations, domain.ProbeConfiguration{Probe: probe, Role: connection.Role, Mode: mode, IsPowerRail: strings.Contains(strings.ToLower(connection.Behavior), "voltage") || strings.Contains(strings.ToLower(connection.Role), "vcc") || strings.Contains(strings.ToLower(connection.Role), "power"), Expected: connection.Expected, SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: scale, Notes: warning}})
		probeNumber++
	}
	if len(configurations) == 0 {
		return domain.ProbePlan{}, nil, errors.New("no profile connection can be mapped to P1-P6")
	}
	for probeNumber <= 6 {
		configurations = append(configurations, domain.ProbeConfiguration{
			Probe: fmt.Sprintf("P%d", probeNumber), Role: "UNASSIGNED", Mode: domain.ProbeModeDigital,
			Expected:        domain.ExpectedSignal{SignalType: "unassigned", Required: false},
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1, Notes: "No target node assigned; leave this probe disconnected."},
		})
		probeNumber++
	}
	return plan, configurations, nil
}

func probeMode(behavior string) domain.ProbeMode {
	lower := strings.ToLower(behavior)
	switch {
	case strings.Contains(lower, "voltage"), strings.Contains(lower, "analog"):
		return domain.ProbeModeAnalog
	case strings.Contains(lower, "pulse"), strings.Contains(lower, "pwm"), strings.Contains(lower, "i2c"), strings.Contains(lower, "uart"), strings.Contains(lower, "clock"):
		return domain.ProbeModePulse
	default:
		return domain.ProbeModeDigital
	}
}

func expectedDescription(expected domain.ExpectedSignal) string {
	if expected.NominalVoltage != nil {
		return fmt.Sprintf("approximately %.2f V", *expected.NominalVoltage)
	}
	if expected.MinVoltage != nil && expected.MaxVoltage != nil {
		return fmt.Sprintf("%.2f–%.2f V", *expected.MinVoltage, *expected.MaxVoltage)
	}
	if expected.NominalFrequencyHz != nil {
		return fmt.Sprintf("approximately %.1f Hz", *expected.NominalFrequencyHz)
	}
	if expected.SignalType != "" {
		return expected.SignalType
	}
	return "user-confirmed signal behavior"
}
