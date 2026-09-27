package projectunderstanding

import (
	"fmt"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// SuggestPhysicalProbeMapping applies the user-confirmed ReWeird S3 hardware
// contract, but only when a real frame confirms all six configured modes.
// It never confirms a profile, treats observed values as expectations, or
// accepts the frame as a measurement under a different profile ID.
func SuggestPhysicalProbeMapping(profile domain.ProjectProfile, frame domain.TelemetryEnvelope) domain.ProjectProfile {
	samples := make(map[string]domain.TelemetrySample, len(frame.Samples))
	for _, sample := range frame.Samples {
		samples[sample.Probe] = sample
	}
	if samples["P1"].Mode != domain.ProbeModeAnalog || samples["P2"].Mode != domain.ProbeModePulse ||
		samples["P3"].Mode != domain.ProbeModePulse || samples["P4"].Mode != domain.ProbeModeDigital ||
		samples["P5"].Mode != domain.ProbeModeAnalog || samples["P6"].Mode != domain.ProbeModeDigital {
		return profile
	}
	roles := map[string]int{}
	for index, connection := range profile.Connections {
		for _, role := range []string{"TRIG", "ECHO", "SERVO", "ZMPT"} {
			if strings.Contains(strings.ToUpper(connection.Role), role) {
				if _, exists := roles[role]; exists {
					return profile // ambiguous code; ask the user rather than guess
				}
				roles[role] = index
			}
		}
	}
	if len(roles) != 4 {
		return profile
	}
	profile.ReservedProbes = []string{"P6"}
	profile.Connections = append([]domain.ProfileConnection(nil), profile.Connections...)
	for index := range profile.Connections {
		profile.Connections[index].Sources = append([]domain.ProjectFactSource(nil), profile.Connections[index].Sources...)
		profile.Connections[index].Evidence = append([]domain.ProfileEvidence(nil), profile.Connections[index].Evidence...)
	}
	for role, probe := range map[string]string{"TRIG": "P2", "ECHO": "P3", "SERVO": "P4", "ZMPT": "P5"} {
		connection := &profile.Connections[roles[role]]
		if connection.Probe != "" && connection.Probe != probe {
			return profile // do not override a user or vision mapping
		}
		connection.Probe = probe
		connection.Role = role
		connection.Sources = appendUniqueSource(connection.Sources, domain.SourceHardwareContract)
		connection.Evidence = append(connection.Evidence, domain.ProfileEvidence{
			Value:  fmt.Sprintf("%s reports %s (%d edges, %d periods, %d analog samples) from physical device %s; placement is a proposal, not wire identification", probe, samples[probe].Mode, samples[probe].EdgeCount, len(samples[probe].PeriodsUS), len(samples[probe].AnalogMV), frame.DeviceID),
			Source: domain.SourceRealSerial, Confidence: 0.65,
		})
	}
	// The target firmware emits a servo PWM signal, but this ReWeird input
	// captures it in digital mode (edges, periods and HIGH widths).
	servo := &profile.Connections[roles["SERVO"]]
	servo.Behavior = "digital_input"
	servo.Expected.SignalType = "servo PWM observed as digital input"
	servo.Expected.Stable = false
	zmpt := &profile.Connections[roles["ZMPT"]]
	zmpt.Behavior = "analog_input"
	zmpt.Expected.SignalType = "analog voltage"
	zmpt.Expected.Required = false // observe without requiring an invented voltage range
	zmpt.Expected.Stable = false

	// Catalog supply specification is an expectation, never an observed
	// baseline. It also tells the probe plan a verified divider is required.
	if !hasRailConnection(profile) && hasComponent(profile, "hc-sr04") {
		profile.Connections = append(profile.Connections, domain.ProfileConnection{
			ID: "suggested-hc-sr04-vcc", ComponentID: "hc-sr04", ComponentName: "HC-SR04", Role: "VCC", Probe: "P1",
			Target: "HC-SR04 VCC / 5 V rail (verify placement)", Direction: "power", Behavior: "analog_input",
			Expected: domain.ExpectedSignal{SignalType: "analog voltage", Required: true, Stable: true, MinVoltage: floatPointer(4.5), MaxVoltage: floatPointer(5.5), NominalVoltage: floatPointer(5), MaxDropouts: 0},
			Sources:  []domain.ProjectFactSource{domain.SourceCatalog, domain.SourceHardwareContract, domain.SourceRealSerial}, Confidence: 0.65,
			Evidence: []domain.ProfileEvidence{{Value: "HC-SR04 catalog: nominal 5 V supply; P1 is the protected rail input under the ReWeird S3 hardware contract", Source: domain.SourceCatalog, Confidence: 0.8}}, Required: true,
		})
	}
	profile.UnresolvedQuestions = append(profile.UnresolvedQuestions,
		"Verify every suggested probe placement against the actual wiring. The serial frame proves signal modes, not which circuit node each probe touches.")
	if frame.ProfileID != profile.ID {
		profile.UnresolvedQuestions = append(profile.UnresolvedQuestions,
			fmt.Sprintf("Firmware reports profile_id %s; reflash with profile_id %s before live diagnostics.", frame.ProfileID, profile.ID))
	}
	return profile
}

func hasComponent(profile domain.ProjectProfile, id string) bool {
	for _, component := range profile.Components {
		if component.ID == id {
			return true
		}
	}
	return false
}

func hasRailConnection(profile domain.ProjectProfile) bool {
	for _, connection := range profile.Connections {
		if connection.Probe == "P1" || strings.EqualFold(connection.Role, "VCC") {
			return true
		}
	}
	return false
}

func floatPointer(value float64) *float64 { return &value }
