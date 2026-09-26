package simulator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
)

const demoWindowMS uint32 = 60_000

const (
	ScenarioHealthy              = "healthy"
	ScenarioDeadSignal           = "dead-signal"
	ScenarioLowVoltage           = "low-voltage"
	ScenarioUnstablePower        = "unstable-power"
	ScenarioMissingPulses        = "missing-pulses"
	ScenarioIntermittent         = "intermittent-connection"
	ScenarioSimultaneousDropouts = "simultaneous-dropouts"
	ScenarioTimingDrift          = "timing-drift"
	ScenarioSoftwareChange       = "software-controlled-change"
)

var scenarios = []domain.SimulatorScenario{
	{ID: ScenarioHealthy, Name: "Healthy", Description: "All monitored signals stay inside the confirmed profile and trusted baselines.", ExpectedFinding: "Signals within configured limits"},
	{ID: ScenarioDeadSignal, Name: "Dead signal", Description: "The required ECHO path has no edges or activity in the measurement window.", ExpectedFinding: "Missing ECHO activity"},
	{ID: ScenarioLowVoltage, Name: "Low voltage", Description: "The measured power rail falls below its trusted specification.", ExpectedFinding: "POWER voltage outside specification"},
	{ID: ScenarioUnstablePower, Name: "Unstable power", Description: "The rail average is plausible but its variation exceeds the allowed tolerance.", ExpectedFinding: "Shared electrical instability"},
	{ID: ScenarioMissingPulses, Name: "Missing pulses", Description: "The ECHO waveform remains present but contains fewer pulses than expected.", ExpectedFinding: "Intermittent ECHO activity"},
	{ID: ScenarioIntermittent, Name: "Intermittent connection", Description: "Isolated ECHO activity drops out in several time buckets and worsens during movement.", ExpectedFinding: "Intermittent ECHO activity"},
	{ID: ScenarioSimultaneousDropouts, Name: "Multiple simultaneous dropouts", Description: "TRIG and ECHO fail in the same buckets, pointing to a shared cause.", ExpectedFinding: "Shared electrical instability"},
	{ID: ScenarioTimingDrift, Name: "Timing drift", Description: "TRIG remains active but its derived frequency moves outside specification with elevated jitter.", ExpectedFinding: "TRIG frequency outside specification"},
	{ID: ScenarioSoftwareChange, Name: "Software-controlled change", Description: "A required signal goes inactive without movement evidence; software state remains a live hypothesis.", ExpectedFinding: "Missing ECHO activity"},
}

type UltrasonicSource struct {
	mu       sync.Mutex
	stage    domain.Stage
	scenario string
	sequence uint64
}

func NewUltrasonicSource() *UltrasonicSource {
	return &UltrasonicSource{stage: domain.StageDiagnose, scenario: ScenarioIntermittent}
}

func (source *UltrasonicSource) Name() string { return "simulator" }

func (source *UltrasonicSource) SetStage(stage domain.Stage) {
	source.mu.Lock()
	source.stage = stage
	source.mu.Unlock()
}

func (source *UltrasonicSource) Scenarios() []domain.SimulatorScenario {
	return append([]domain.SimulatorScenario(nil), scenarios...)
}

func (source *UltrasonicSource) SetScenario(id string) error {
	if !knownScenario(id) {
		return fmt.Errorf("unknown simulator scenario %q", id)
	}
	source.mu.Lock()
	source.scenario = id
	source.stage = domain.StageDiagnose
	source.mu.Unlock()
	return nil
}

func (source *UltrasonicSource) CurrentScenario() string {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.scenario
}

func (source *UltrasonicSource) Latest(_ context.Context) (domain.TelemetryEnvelope, error) {
	source.mu.Lock()
	source.sequence++
	stage := source.stage
	scenario := source.scenario
	sequence := source.sequence
	source.mu.Unlock()
	return frameForScenario(scenario, stage, sequence), nil
}

func frameForScenario(scenario string, stage domain.Stage, sequence uint64) domain.TelemetryEnvelope {
	if stage == domain.StageVerify {
		scenario = ScenarioHealthy
	}

	power := []float64{2505, 2500, 2510, 2505, 2505, 2500, 2510, 2505, 2500, 2505, 2510, 2505}
	trigPeriods := []float64{25.06, 24.94, 25, 24.88, 25.06, 25, 24.94, 25}
	trigWidths := []float64{10, 10.1, 9.9, 10, 10, 9.9, 10.1, 10}
	trigActivity := []float64{39.9, 40.1, 40, 40.2, 39.9, 40, 40.1, 40, 39.8, 40.1, 40, 40.2}
	trigEdges := uint32(2_400_000)
	echoActivity := []float64{28, 29, 28, 29, 28, 29, 28, 28, 29, 28, 29, 28}
	echoPulses := uint32(1704)
	echoPeriods := []float64{35_100, 35_260, 35_180, 35_300, 35_080, 35_210, 35_290, 35_160}
	echoWidths := []float64{1450, 1520, 1490, 1505, 1475, 1510, 1485, 1500}
	var trigMaxGap, echoMaxGap uint64

	switch scenario {
	case ScenarioDeadSignal, ScenarioSoftwareChange:
		echoActivity = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		echoPulses = 0
		echoPeriods = nil
		echoWidths = nil
		echoMaxGap = uint64(demoWindowMS) * 1000
	case ScenarioLowVoltage:
		power = []float64{1810, 1800, 1795, 1805, 1800, 1790, 1805, 1800, 1795, 1805, 1800, 1790}
	case ScenarioUnstablePower:
		power = []float64{2500, 2700, 2320, 2650, 2380, 2720, 2300, 2600, 2400, 2680, 2350, 2500}
	case ScenarioMissingPulses:
		echoActivity = []float64{25, 24, 25, 24, 25, 24, 25, 24, 25, 24, 25, 24}
		echoPulses = 1470
		echoMaxGap = 140_000
	case ScenarioIntermittent:
		echoActivity = []float64{29, 28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27}
		echoPulses = 1692
		echoMaxGap = 175_000
		if stage == domain.StageTest || stage == domain.StageRepair {
			echoActivity = []float64{28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0}
			echoPulses = 1677
			echoMaxGap = 420_000
		}
	case ScenarioSimultaneousDropouts:
		trigActivity = []float64{40, 40, 0, 40, 40, 0, 40, 40, 40, 0, 40, 40}
		echoActivity = []float64{28, 29, 0, 28, 29, 0, 28, 29, 28, 0, 29, 28}
		trigMaxGap = 75
		echoMaxGap = 105_000
	case ScenarioTimingDrift:
		trigPeriods = []float64{31.0, 34.8, 29.5, 36.2, 32.1, 35.4, 30.2, 37.0}
		trigWidths = []float64{11.8, 13.2, 10.7, 14.1, 12.4, 13.7, 11.2, 14.5}
	}

	return domain.TelemetryEnvelope{
		SchemaVersion: telemetry.SchemaVersion,
		DeviceID:      "reweird-simulator-001",
		ProfileID:     "ultrasonic-demo",
		CapturedAtMS:  time.Now().UTC().UnixMilli(),
		UptimeMS:      sequence * uint64(demoWindowMS),
		WindowMS:      demoWindowMS,
		Sequence:      sequence,
		Samples: []domain.TelemetrySample{
			{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: power, ActivityCounts: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
			{Probe: "P2", Mode: domain.ProbeModePulse, EdgeCount: trigEdges * 2, RisingEdges: trigEdges, FallingEdges: trigEdges, PeriodsUS: trigPeriods, HighPulseWidthsUS: trigWidths, MaxGapUS: trigMaxGap, ActivityCounts: trigActivity},
			{Probe: "P3", Mode: domain.ProbeModePulse, EdgeCount: echoPulses * 2, RisingEdges: echoPulses, FallingEdges: echoPulses, PeriodsUS: echoPeriods, HighPulseWidthsUS: echoWidths, MaxGapUS: echoMaxGap, ActivityCounts: echoActivity},
			digitalSample("P4"), digitalSample("P5"), digitalSample("P6"),
		},
	}
}

func knownScenario(id string) bool {
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return true
		}
	}
	return false
}

func digitalSample(probe string) domain.TelemetrySample {
	state := 0
	return domain.TelemetrySample{Probe: probe, Mode: domain.ProbeModeDigital, State: &state}
}
