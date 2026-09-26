package httpapi

import (
	"net/http"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/computer"
)

func TestComputerSimulationAndDefaultLockedCollector(t *testing.T) {
	t.Setenv("COMPUTER_DIAGNOSTICS_ENABLED", "false")
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/computer/scenarios", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("scenarios status = %d", response.StatusCode)
	}
	var scenarios struct {
		Scenarios []computer.Scenario `json:"scenarios"`
	}
	decodeBody(t, response, &scenarios)
	if len(scenarios.Scenarios) != 8 {
		t.Fatalf("scenarios = %d", len(scenarios.Scenarios))
	}
	response = doJSON(t, app, http.MethodPost, "/api/v1/computer/simulate", map[string]string{"scenario_id": computer.ScenarioPortConflict})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("simulate status = %d", response.StatusCode)
	}
	var analysis computer.Analysis
	decodeBody(t, response, &analysis)
	if len(analysis.Findings) != 1 || analysis.Findings[0].Code != "PORT_CONFLICT_DETECTED" || len(analysis.Evidence.PhysicalEvidence) != 0 {
		t.Fatalf("analysis = %+v", analysis)
	}
	response = doJSON(t, app, http.MethodPost, "/api/v1/computer/collect", map[string]int{"exclusive_port": 3000})
	if response.StatusCode != http.StatusLocked {
		t.Fatalf("real collector default status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
}
