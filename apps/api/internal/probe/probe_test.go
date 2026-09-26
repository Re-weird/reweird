package probe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func deterministicDiagnosis() domain.Diagnosis {
	return domain.Diagnosis{
		Headline:       "Intermittent ECHO activity",
		Summary:        "The failure is isolated to the ECHO path.",
		PossibleCauses: []string{"Intermittent connection"},
		Confidence:     0.68,
		NextTest:       "Gently wiggle the ECHO connection.",
	}
}

func TestNewGeminiUsesConfiguredProviderAndCurrentDefault(t *testing.T) {
	if _, ok := NewGemini("", "").(MockProvider); !ok {
		t.Fatal("empty API key should select the mock provider")
	}
	provider, ok := NewGemini("configured-key", "").(*GeminiProvider)
	if !ok {
		t.Fatal("configured API key should select the Gemini provider")
	}
	if provider.model != "gemini-3.5-flash-lite" || !strings.Contains(provider.endpoint, "/v1beta/models/gemini-3.5-flash-lite:generateContent") {
		t.Fatalf("configured Gemini provider = %#v", provider)
	}
}

func TestMockProviderReturnsDeterministicDiagnosisUnchanged(t *testing.T) {
	deterministic := deterministicDiagnosis()
	result := MockProvider{}.Diagnose(context.Background(), domain.Evidence{}, deterministic)
	if !reflect.DeepEqual(result, deterministic) {
		t.Fatalf("MockProvider.Diagnose() = %#v, want unchanged %#v", result, deterministic)
	}
}

func TestGeminiProviderRewordsWithinDeterministicConfidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatal("missing API key header")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		requestJSON := string(body)
		if !strings.Contains(requestJSON, `"responseSchema"`) || !strings.Contains(requestJSON, `"application/json"`) {
			t.Fatalf("Gemini request is missing structured-output configuration: %s", requestJSON)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"headline\":\"Likely loose ECHO wire\",\"summary\":\"Plain-English explanation.\",\"possible_causes\":[\"Loose jumper\"],\"confidence\":0.99,\"next_test\":\"Wiggle it.\"}"}]}}]}`))
	}))
	defer server.Close()
	deterministic := deterministicDiagnosis()
	result := NewGeminiForTest("test-key", "test-model", server.URL, server.Client()).Diagnose(context.Background(), domain.Evidence{Probe: "P3", Role: "ECHO"}, deterministic)
	if result.Headline != "Likely loose ECHO wire" {
		t.Fatalf("Diagnose() headline = %q", result.Headline)
	}
	if result.Confidence != deterministic.Confidence {
		t.Fatalf("Diagnose() confidence = %v, want capped at deterministic %v", result.Confidence, deterministic.Confidence)
	}
}

func TestGeminiProviderFallsBackOnTransportFailure(t *testing.T) {
	deterministic := deterministicDiagnosis()
	provider := NewGeminiForTest("test-key", "test-model", "http://127.0.0.1:0", http.DefaultClient)
	result := provider.Diagnose(context.Background(), domain.Evidence{}, deterministic)
	if !reflect.DeepEqual(result, deterministic) {
		t.Fatalf("Diagnose() = %#v, want fallback to deterministic %#v", result, deterministic)
	}
}

func TestGeminiProviderFallsBackOnEmptyHeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"headline\":\"\",\"summary\":\"x\",\"possible_causes\":[],\"confidence\":0.5,\"next_test\":\"x\"}"}]}}]}`))
	}))
	defer server.Close()
	deterministic := deterministicDiagnosis()
	result := NewGeminiForTest("test-key", "test-model", server.URL, server.Client()).Diagnose(context.Background(), domain.Evidence{}, deterministic)
	if !reflect.DeepEqual(result, deterministic) {
		t.Fatalf("Diagnose() = %#v, want fallback to deterministic %#v", result, deterministic)
	}
}
