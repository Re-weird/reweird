package simulator

import (
	"context"
	"sync"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

const demoWindowMS uint32 = 60_000

type UltrasonicSource struct {
	mu       sync.RWMutex
	stage    domain.Stage
	sequence uint64
}

func NewUltrasonicSource() *UltrasonicSource {
	return &UltrasonicSource{stage: domain.StageDiagnose}
}

func (source *UltrasonicSource) Name() string { return "simulator" }

func (source *UltrasonicSource) SetStage(stage domain.Stage) {
	source.mu.Lock()
	source.stage = stage
	source.sequence++
	source.mu.Unlock()
}

func (source *UltrasonicSource) Latest(_ context.Context) (domain.TelemetryEnvelope, error) {
	source.mu.RLock()
	stage := source.stage
	sequence := source.sequence
	source.mu.RUnlock()
	return frameForStage(stage, sequence), nil
}

func frameForStage(stage domain.Stage, sequence uint64) domain.TelemetryEnvelope {
	verified := stage == domain.StageVerify
	wiggling := stage == domain.StageTest || stage == domain.StageRepair
	echoActivity := []float64{29, 28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27}
	echoPulses := uint32(1692)
	if wiggling {
		echoActivity = []float64{28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0}
		echoPulses = 1677
	}
	if verified {
		echoActivity = []float64{28, 29, 28, 29, 28, 29, 28, 28, 29, 28, 29, 28}
		echoPulses = 1704
	}

	return domain.TelemetryEnvelope{
		SchemaVersion: 1,
		DeviceID:      "reweird-simulator-001",
		CapturedAtMS:  time.Now().UTC().UnixMilli(),
		UptimeMS:      sequence * uint64(demoWindowMS),
		WindowMS:      demoWindowMS,
		Sequence:      sequence,
		Samples: []domain.TelemetrySample{
			{
				Probe:          "P1",
				Mode:           domain.ProbeModeAnalog,
				AnalogMV:       []float64{2505, 2500, 2510, 2505, 2505, 2500, 2510, 2505, 2500, 2505, 2510, 2505},
				ActivityCounts: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			},
			{
				Probe:             "P2",
				Mode:              domain.ProbeModePulse,
				EdgeCount:         4_800_000,
				RisingEdges:       2_400_000,
				FallingEdges:      2_400_000,
				PeriodsUS:         []float64{25.06, 24.94, 25, 24.88, 25.06, 25, 24.94, 25},
				HighPulseWidthsUS: []float64{10, 10.1, 9.9, 10, 10, 9.9, 10.1, 10},
				ActivityCounts:    []float64{39.9, 40.1, 40, 40.2, 39.9, 40, 40.1, 40, 39.8, 40.1, 40, 40.2},
			},
			{
				Probe:             "P3",
				Mode:              domain.ProbeModePulse,
				EdgeCount:         echoPulses * 2,
				RisingEdges:       echoPulses,
				FallingEdges:      echoPulses,
				PeriodsUS:         []float64{35_100, 35_260, 35_180, 35_300, 35_080, 35_210, 35_290, 35_160},
				HighPulseWidthsUS: []float64{1450, 1520, 1490, 1505, 1475, 1510, 1485, 1500},
				ActivityCounts:    echoActivity,
			},
			digitalSample("P4"),
			digitalSample("P5"),
			digitalSample("P6"),
		},
	}
}

func digitalSample(probe string) domain.TelemetrySample {
	state := 0
	return domain.TelemetrySample{Probe: probe, Mode: domain.ProbeModeDigital, State: &state}
}
