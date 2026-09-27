// Package physicalfixture provides test fixtures built from real Telemetry v2
// frames captured from the ESP32-S3 probe board (device reweird-3428B5AD4F7C)
// on the HC-SR04 + servo bench circuit. It is imported only by tests.
package physicalfixture

import "github.com/re-weird/reweird/apps/api/internal/domain"

const DeviceID = "reweird-3428B5AD4F7C"
const ProfileID = "ultrasonic-servo-bench"

func number(value float64) *float64 { return &value }

// Profile describes the bench circuit as a project profile would after the
// user confirms it. Expectations come from test_circuit.ino (10 µs trigger,
// 250 ms loop delay, 50 Hz servo with 500–2400 µs pulses) and the HC-SR04
// datasheet (5 V supply, echo up to the 30 ms pulseIn timeout).
func Profile() domain.ProjectProfile {
	return domain.ProjectProfile{
		ID: ProfileID, ProjectID: ProfileID, Version: 1, ProjectName: "Ultrasonic servo bench",
		Controller: "ESP32", LogicVoltage: 3.3, Confirmed: true, ConfirmedBy: "user",
		Probes: []domain.ProbeConfiguration{
			{Probe: "P1", Role: "POWER", Mode: domain.ProbeModeAnalog, IsPowerRail: true,
				Expected:        domain.ExpectedSignal{SignalType: "voltage rail", Required: true, Stable: true, MinVoltage: number(4.75), MaxVoltage: number(5.25), NominalVoltage: number(5), VoltageTolerancePct: 5},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 2}},
			{Probe: "P2", Role: "TRIG", Mode: domain.ProbeModePulse,
				Expected:        domain.ExpectedSignal{SignalType: "pulse", Required: true, MinFrequencyHz: number(3.4), MaxFrequencyHz: number(4.0), MinPulseWidthUS: number(5), MaxPulseWidthUS: number(20)},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}},
			{Probe: "P3", Role: "ECHO", Mode: domain.ProbeModePulse,
				Expected:        domain.ExpectedSignal{SignalType: "return pulse", Required: true, MinFrequencyHz: number(3.4), MaxFrequencyHz: number(4.0), MaxPulseWidthUS: number(38_000)},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}},
			{Probe: "P4", Role: "SERVO", Mode: domain.ProbeModeDigital,
				Expected:        domain.ExpectedSignal{SignalType: "pwm", Required: true, MinFrequencyHz: number(45), MaxFrequencyHz: number(55), MinPulseWidthUS: number(400), MaxPulseWidthUS: number(2600)},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}},
			// P5 watches ZMPT OUT (user-confirmed, analog). Its healthy level
			// is not known from the code, so it is observed but not required.
			{Probe: "P5", Role: "ZMPT", Mode: domain.ProbeModeAnalog,
				Expected:        domain.ExpectedSignal{SignalType: "analog level"},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}},
			unassigned("P6"),
		},
	}
}

func unassigned(probe string) domain.ProbeConfiguration {
	return domain.ProbeConfiguration{Probe: probe, Role: "UNASSIGNED", Mode: domain.ProbeModeDigital,
		Expected:        domain.ExpectedSignal{SignalType: "unassigned"},
		SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}}
}

func repeat(value float64, count int) []float64 {
	values := make([]float64, count)
	for index := range values {
		values[index] = value
	}
	return values
}

// Frame is a healthy one-second window shaped like the corrected firmware's
// output: ECHO and servo values are the ones measured on the bench; TRIG is
// the 10 µs pulse the hardware capture now resolves.
func Frame(sequence uint64) domain.TelemetryEnvelope {
	zero := 0
	return domain.TelemetryEnvelope{
		SchemaVersion: 2, DeviceID: DeviceID, ProfileID: ProfileID, CapturedAtMS: 0,
		UptimeMS: 486_345 + sequence*1000, WindowMS: 1000, Sequence: sequence,
		Samples: []domain.TelemetrySample{
			{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{2538, 2534, 2535, 2530, 2535, 2534, 2538, 2534, 2535, 2529, 2535, 2535, 2534, 2533, 2527, 2532}},
			{Probe: "P2", Mode: domain.ProbeModePulse, State: &zero, EdgeCount: 8, RisingEdges: 4, FallingEdges: 4, PeriodsUS: []float64{265_002, 264_999, 265_001}, HighPulseWidthsUS: []float64{10.1, 10.0, 10.1, 10.0}, MaxGapUS: 264_990, ActivityCounts: []float64{1, 0, 1, 0, 0, 1, 0, 0, 1, 0}},
			{Probe: "P3", Mode: domain.ProbeModePulse, State: &zero, EdgeCount: 8, RisingEdges: 4, FallingEdges: 4, PeriodsUS: []float64{265_002, 264_999, 265_001}, HighPulseWidthsUS: []float64{12_296, 12_273, 12_268, 12_270}, MaxGapUS: 161_244, ActivityCounts: []float64{1, 0, 1, 0, 0, 1, 0, 0, 1, 0}},
			{Probe: "P4", Mode: domain.ProbeModeDigital, State: &zero, EdgeCount: 100, RisingEdges: 50, FallingEdges: 50, PeriodsUS: repeat(20_000, 32), HighPulseWidthsUS: repeat(486.5, 32), MaxGapUS: 18_845, ActivityCounts: repeat(5, 10)},
			{Probe: "P5", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{1651, 1653, 1656, 1652, 1650, 1654}},
			{Probe: "P6", Mode: domain.ProbeModeDigital, State: &zero, MaxGapUS: 1_000_000, ActivityCounts: repeat(0, 10)},
		},
	}
}

// LegacyTrigSample is P2 exactly as the previous interrupt firmware reported
// it (measurement #340): a falling edge without its rising edge and a "high
// width" that started 42 s earlier in another window.
func LegacyTrigSample() domain.TelemetrySample {
	zero := 0
	return domain.TelemetrySample{Probe: "P2", Mode: domain.ProbeModePulse, State: &zero, EdgeCount: 1, FallingEdges: 1, HighPulseWidthsUS: []float64{42_652_091}, MaxGapUS: 705_684, ActivityCounts: repeat(0, 10)}
}
