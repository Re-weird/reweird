package projectunderstanding

import (
	"context"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

func TestSerialProposalKeepsPhysicalModesTentative(t *testing.T) {
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	service := New(codeanalysis.New(), fakeVision{}, catalog)
	project := domain.Project{ID: "my-physical-project", Name: "Physical bench", Controller: "ESP32", LogicVoltage: 3.3,
		Code: &domain.ProjectCode{Filename: "test_circuit.ino", Text: `#include <ESP32Servo.h>
const int TRIG_PIN = 5;
const int ECHO_PIN = 18;
const int SERVO_PIN = 23;
const int ZMPT_PIN = 34;
void setup(){ pinMode(TRIG_PIN, OUTPUT); pinMode(ECHO_PIN, INPUT); myServo.attach(SERVO_PIN, 500, 2400); }
void loop(){ digitalWrite(TRIG_PIN, HIGH); pulseIn(ECHO_PIN, HIGH); analogRead(ZMPT_PIN); }`}}
	_, draft := service.Analyze(context.Background(), project, "")
	frame := physicalfixture.Frame(10)
	frame.ProfileID = "ultrasonic-demo"
	proposal := SuggestPhysicalProbeMapping(draft, frame)
	if proposal.Confirmed || len(proposal.Connections) != 5 {
		t.Fatalf("proposal must remain a five-connection draft: %+v", proposal)
	}
	for _, want := range []struct{ role, probe, behavior string }{{"VCC", "P1", "analog_input"}, {"TRIG", "P2", "digital_pulse"}, {"ECHO", "P3", "pulse_input"}, {"SERVO", "P4", "digital_input"}, {"ZMPT", "P5", "analog_input"}} {
		found := false
		for _, connection := range proposal.Connections {
			if connection.Role == want.role && connection.Probe == want.probe && connection.Behavior == want.behavior {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s/%s/%s: %+v", want.role, want.probe, want.behavior, proposal.Connections)
		}
	}
	for _, connection := range proposal.Connections {
		if connection.Probe == "P6" {
			t.Fatal("P6 must remain unassigned")
		}
	}
	if !strings.Contains(strings.Join(proposal.UnresolvedQuestions, " "), "reflash") {
		t.Fatal("missing firmware profile-ID warning")
	}
	if len(proposal.ReservedProbes) != 1 || proposal.ReservedProbes[0] != "P6" {
		t.Fatal("P6 was not reserved as spare")
	}
	proposal.Connections = append(proposal.Connections, domain.ProfileConnection{ID: "extra-led", Role: "LED", ComponentName: "LED", Target: "extra GPIO", Behavior: "digital_input", Expected: domain.ExpectedSignal{SignalType: "digital"}})
	plan, probes, err := GenerateProbePlan(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Instructions) != 6 || probes[4].Mode != domain.ProbeModeAnalog || probes[5].Role != "UNASSIGNED" || probes[0].SafeMeasurement.InputScale != 2 {
		t.Fatalf("unsafe generated plan: %+v / %+v", plan, probes)
	}

	frame.Samples[4].Mode = domain.ProbeModeDigital
	if changed := SuggestPhysicalProbeMapping(draft, frame); len(changed.Connections) != len(draft.Connections) || changed.Connections[0].Probe != "" {
		t.Fatal("mode-mismatched telemetry must not invent a mapping")
	}
}
