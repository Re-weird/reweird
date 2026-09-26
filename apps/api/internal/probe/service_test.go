package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func mainEvidence() domain.Evidence {
	return domain.Evidence{
		Probe:    "P3",
		Role:     "ECHO",
		Expected: map[string]any{"signal": "pulse", "required": true, "max_dropouts": 0},
		Observed: map[string]any{"dropouts_per_window": 12, "stable": false},
		Baseline: map[string]any{"status": domain.BaselineUserConfirmedHealthy, "trusted": true},
		Measurements: []domain.EvidenceFact{{
			Probe: "P3", Name: "pulse_count", Value: 1450, Provenance: domain.ProvenanceMeasured,
		}},
		DerivedFacts: []domain.EvidenceFact{{
			Probe: "P3", Name: "dropout_events", Value: 12, Provenance: domain.ProvenanceDerived,
		}},
		RuleResults: []domain.RuleResult{{
			ID: "unexpected-dropout", Probe: "P3", Status: "fail", Severity: 3,
			Message: "12 unexpected ECHO dropouts detected", Provenance: domain.ProvenanceDerived,
		}},
	}
}

func equalJSON(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func TestServiceProviderSendsMainContractsAndMapsDiagnosis(t *testing.T) {
	deterministic := deterministicDiagnosis()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/probe/main-diagnosis" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var payload struct {
			Evidence               domain.Evidence  `json:"evidence"`
			DeterministicDiagnosis domain.Diagnosis `json:"deterministic_diagnosis"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !equalJSON(payload.Evidence, mainEvidence()) {
			t.Fatalf("evidence changed across boundary: %#v", payload.Evidence)
		}
		if !reflect.DeepEqual(payload.DeterministicDiagnosis, deterministic) {
			t.Fatalf("deterministic diagnosis changed: %#v", payload.DeterministicDiagnosis)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(domain.Diagnosis{
			Headline:       "Unexpected signal dropout",
			Summary:        "ECHO (P3): 12 unexpected ECHO dropouts detected",
			PossibleCauses: []string{"Unexpected signal dropout"},
			Confidence:     0.68,
			NextTest:       "Run a controlled movement test.",
		})
	}))
	defer server.Close()

	result := NewServiceForTest(server.URL+"/probe/main-diagnosis", server.Client()).Diagnose(
		context.Background(), mainEvidence(), deterministic,
	)
	if result.Headline != "Unexpected signal dropout" || result.NextTest != "Run a controlled movement test." {
		t.Fatalf("Diagnose() = %#v", result)
	}
}

func TestServiceProviderFailsClosedOnInvalidOrUnavailableResponse(t *testing.T) {
	deterministic := deterministicDiagnosis()
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "status", handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusServiceUnavailable)
		})},
		{name: "malformed", handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte(`{"headline":`))
		})},
		{name: "trailing JSON", handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte(`{"headline":"x","summary":"x","possible_causes":[],"confidence":0.5,"next_test":"x"}{}`))
		})},
		{name: "confidence escalation", handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(response).Encode(domain.Diagnosis{
				Headline: "x", Summary: "x", Confidence: 0.99, NextTest: "x",
			})
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			result := NewServiceForTest(server.URL, server.Client()).Diagnose(
				context.Background(), mainEvidence(), deterministic,
			)
			if !reflect.DeepEqual(result, deterministic) {
				t.Fatalf("Diagnose() = %#v, want deterministic fallback %#v", result, deterministic)
			}
		})
	}
}

func TestNewServiceWithoutURLUsesDeterministicFallback(t *testing.T) {
	if _, ok := NewService(" ").(MockProvider); !ok {
		t.Fatal("empty PROBE service URL must select the deterministic fallback")
	}
}
