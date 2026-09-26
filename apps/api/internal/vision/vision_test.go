package vision

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiVisionForcesAIProvenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatal("missing API key header")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		requestJSON := string(body)
		if !strings.Contains(requestJSON, `"inlineData"`) || !strings.Contains(requestJSON, `"responseSchema"`) || !strings.Contains(requestJSON, `"application/json"`) {
			t.Fatalf("Gemini request is missing image or structured-output configuration: %s", requestJSON)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"components\":[{\"catalog_id\":\"hc-sr04\",\"name\":\"HC-SR04\",\"confidence\":0.9,\"visible_labels\":[\"HC-SR04\"]}],\"relationships\":[],\"warnings\":[]}"}]}}]}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, 0xd9}, 0o600); err != nil {
		t.Fatal(err)
	}
	result := NewGeminiForTest("test-key", "test-model", server.URL, server.Client()).Analyze(context.Background(), path, "image/jpeg")
	if result.Status != "VISION_COMPLETE" || len(result.Components) != 1 || result.Components[0].Source != "VISION_AI" {
		t.Fatalf("Analyze() = %#v", result)
	}
}

func TestNewGeminiUsesConfiguredAnalyzerAndCurrentDefault(t *testing.T) {
	if _, ok := NewGemini("", "").(SkippedAnalyzer); !ok {
		t.Fatal("empty API key should select the skipped analyzer")
	}
	analyzer, ok := NewGemini("configured-key", "").(*GeminiAnalyzer)
	if !ok {
		t.Fatal("configured API key should select the Gemini analyzer")
	}
	if analyzer.model != "gemini-3.5-flash-lite" || !strings.Contains(analyzer.endpoint, "/v1beta/models/gemini-3.5-flash-lite:generateContent") {
		t.Fatalf("configured Gemini analyzer = %#v", analyzer)
	}
}
