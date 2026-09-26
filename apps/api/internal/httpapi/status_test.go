package httpapi

import (
	"net/http"
	"testing"
)

func TestSystemStatusDoesNotExposeConfigurationSecrets(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "sensitive-status-key-123")
	t.Setenv("REWEIRD_GIT_REPO", "C:/example")
	t.Setenv("REWEIRD_GIT_SYNC_ENABLED", "false")
	t.Setenv("COMPUTER_DIAGNOSTICS_ENABLED", "false")
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/status", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var payload struct {
		API            string `json:"api"`
		Database       string `json:"database"`
		TelemetryMode  string `json:"telemetry_mode"`
		Gemini         string `json:"gemini"`
		Patch          string `json:"patch"`
		GitSyncEnabled bool   `json:"git_sync_enabled"`
		ComputerAgent  string `json:"computer_agent"`
	}
	decodeBody(t, response, &payload)
	if payload.API != "ok" || payload.Database != "ok" || payload.TelemetryMode != "simulator" || payload.Gemini != "configured_vision_only" || payload.Patch != "locked" || payload.GitSyncEnabled || payload.ComputerAgent != "not_running" {
		t.Fatalf("unexpected status %+v", payload)
	}
}
