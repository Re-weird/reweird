package httpapi

import (
	"net/http"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestDemoProbePlanComesFromConfirmedProfile(t *testing.T) {
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/demo/probe-plan", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	var plan domain.ProbePlan
	decodeBody(t, response, &plan)
	if plan.ProfileID != "ultrasonic-demo" || len(plan.Instructions) < 4 || plan.Instructions[0].Probe != "GND" {
		t.Fatalf("plan %+v", plan)
	}
}
