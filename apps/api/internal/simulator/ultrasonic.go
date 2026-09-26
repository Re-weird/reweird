package simulator

import "github.com/re-weird/reweird/apps/api/internal/domain"

type UltrasonicSource struct{}

func NewUltrasonicSource() *UltrasonicSource { return &UltrasonicSource{} }

func value(number float64) *float64 { return &number }

func (source *UltrasonicSource) Snapshot(stage domain.Stage) []domain.ProbeReading {
	verified := stage == domain.StageVerify
	wiggling := stage == domain.StageTest || stage == domain.StageRepair

	echoSamples := []float64{29, 28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27}
	echoValue := 19.7
	dropouts := 12
	status := "intermittent"
	if wiggling {
		echoSamples = []float64{28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0}
		dropouts = 27
	}
	if verified {
		echoSamples = []float64{28, 29, 28, 29, 28, 29, 28, 28, 29, 28, 29, 28}
		echoValue = 28.4
		dropouts = 0
		status = "stable"
	}

	return []domain.ProbeReading{
		{Probe: "P1", Role: "POWER", Value: value(5.01), Unit: "V", Status: "stable", Samples: []float64{5.01, 5, 5.02, 5.01, 5.01, 5, 5.02, 5.01, 5, 5.01, 5.02, 5.01}},
		{Probe: "P2", Role: "TRIG", Value: value(40), Unit: "kHz", Status: "active", Samples: []float64{39.9, 40.1, 40, 40.2, 39.9, 40, 40.1, 40, 39.8, 40.1, 40, 40.2}},
		{Probe: "P3", Role: "ECHO", Value: value(echoValue), Unit: "pulses/s", Status: status, Dropouts: dropouts, Samples: echoSamples},
		{Probe: "P4", Role: "UNASSIGNED", Value: nil, Unit: "", Status: "idle", Samples: []float64{}},
	}
}
