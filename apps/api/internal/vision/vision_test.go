package vision

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiVisionForcesAIProvenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatal("missing API key header")
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
