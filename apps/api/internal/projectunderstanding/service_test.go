package projectunderstanding

import (
	"context"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

type fakeVision struct{ result domain.VisionAnalysis }

func (fake fakeVision) Analyze(context.Context, string, string) domain.VisionAnalysis {
	return fake.result
}

func TestDraftProfileAndConflict(t *testing.T) {
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	visionResult := domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: .94, Source: domain.SourceVisionAI}}, Relationships: []domain.VisionRelationship{{From: "HC-SR04", To: "ESP32", Role: "ECHO", GPIO: integer(17), Confidence: .72, Source: domain.SourceVisionAI}}}
	service := New(codeanalysis.New(), fakeVision{visionResult}, catalog)
	project := domain.Project{ID: "distance-alarm-abc123", Name: "Distance Alarm", Description: "Measure distance", Controller: "ESP32", LogicVoltage: 3.3, Code: &domain.ProjectCode{Filename: "main.ino", Text: "#define TRIG 5\n#define ECHO 18\nvoid setup(){ pinMode(TRIG, OUTPUT); pinMode(ECHO, INPUT); }\nlong x=pulseIn(ECHO,HIGH);"}, Image: &domain.ProjectMedia{ContentType: "image/jpeg"}}
	analysis, profile := service.Analyze(context.Background(), project, "ignored.jpg")
	if len(analysis.Code.Pins) != 2 || len(profile.Components) == 0 || len(profile.Connections) != 2 {
		t.Fatalf("profile = %#v analysis = %#v", profile, analysis)
	}
	if len(profile.Conflicts) != 1 || profile.Conflicts[0].Resolved {
		t.Fatalf("conflicts = %#v", profile.Conflicts)
	}
}

func TestGenericProbePlanUsesProfileOrder(t *testing.T) {
	profile := domain.ProjectProfile{ID: "motor-project", ProjectID: "motor-project", Connections: []domain.ProfileConnection{{ID: "motor", ComponentName: "Motor driver", Role: "MOTOR_PWM", Target: "ESP32 GPIO4 / driver PWM", Behavior: "pwm_output", Expected: domain.ExpectedSignal{SignalType: "PWM", Required: true}}}}
	plan, probes, err := GenerateProbePlan(profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Instructions) != 2 || plan.Instructions[1].Probe != "P1" || plan.Instructions[1].Role != "MOTOR_PWM" || probes[0].Mode != domain.ProbeModePulse {
		t.Fatalf("plan = %#v probes = %#v", plan, probes)
	}
}
