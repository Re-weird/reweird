package projectunderstanding

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestProbePlanHonorsPinnedPhysicalProbes(t *testing.T) {
	five := 5.0
	profile := domain.ProjectProfile{ID: "bench", ProjectID: "bench", Connections: []domain.ProfileConnection{
		{ID: "servo", ComponentName: "Servo", Role: "SERVO", Behavior: "pwm output", Probe: "P4", Expected: domain.ExpectedSignal{SignalType: "pwm", Required: true}},
		{ID: "vcc", ComponentName: "HC-SR04", Role: "VCC", Behavior: "voltage rail", Probe: "P1", Expected: domain.ExpectedSignal{SignalType: "voltage rail", Required: true, NominalVoltage: &five}},
		{ID: "echo", ComponentName: "HC-SR04", Role: "ECHO", Behavior: "pulse input", Expected: domain.ExpectedSignal{SignalType: "pulse", Required: true}},
	}}
	plan, probes, err := GenerateProbePlan(profile)
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, probe := range probes {
		roles[probe.Probe] = probe.Role
	}
	if roles["P1"] != "VCC" || roles["P4"] != "SERVO" || roles["P2"] != "ECHO" || roles["P3"] != "UNASSIGNED" || len(probes) != 6 {
		t.Fatalf("probe mapping = %v", roles)
	}
	if probes[0].SafeMeasurement.InputScale != 2 {
		t.Fatalf("5 V rail lost its divider scale: %+v", probes[0])
	}
	if plan.Instructions[1].Probe != "P1" {
		t.Fatalf("instructions not ordered by probe: %+v", plan.Instructions)
	}
	profile.Connections[2].Probe = "P4"
	if _, _, err := GenerateProbePlan(profile); err == nil {
		t.Fatal("two connections pinned to P4 were accepted")
	}
}
