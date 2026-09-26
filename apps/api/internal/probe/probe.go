// Package probe interprets deterministic diagnostic evidence into
// human-readable findings. It never sees raw telemetry and can never
// overwrite measured, specification, or baseline evidence -- it only
// rewrites the narrative fields of a Diagnosis that the deterministic
// engine already computed.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// Provider turns deterministic evidence + a rule-based diagnosis into a
// (possibly reworded) Diagnosis. Implementations must treat evidence as
// read-only and must fall back to the deterministic diagnosis on any
// failure rather than returning an error that would break the pipeline.
type Provider interface {
	Diagnose(ctx context.Context, evidence domain.Evidence, deterministic domain.Diagnosis) domain.Diagnosis
}

// MockProvider is the deterministic fallback: it returns the rule engine's
// diagnosis unchanged. Used whenever no Gemini API key is configured.
type MockProvider struct{}

func (MockProvider) Diagnose(_ context.Context, _ domain.Evidence, deterministic domain.Diagnosis) domain.Diagnosis {
	return deterministic
}

const maxGeminiResponseBytes = 1024 * 1024

type GeminiProvider struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

// NewGemini returns a Gemini-backed Provider, or MockProvider when no key
// is configured -- mirrors internal/vision's key-gated adapter selection.
func NewGemini(apiKey, model string) Provider {
	if strings.TrimSpace(apiKey) == "" {
		return MockProvider{}
	}
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	return &GeminiProvider{
		apiKey:   apiKey,
		model:    model,
		endpoint: "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent",
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func NewGeminiForTest(apiKey, model, endpoint string, client *http.Client) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey, model: model, endpoint: endpoint, client: client}
}

type geminiProbePayload struct {
	Headline       string   `json:"headline"`
	Summary        string   `json:"summary"`
	PossibleCauses []string `json:"possible_causes"`
	Confidence     float64  `json:"confidence"`
	NextTest       string   `json:"next_test"`
}

// Diagnose asks Gemini to reword the deterministic finding in plain
// English. The deterministic evidence and diagnosis are the only source
// of truth passed in; Gemini is instructed to explain them, not invent
// new facts. Any request, transport, or validation failure falls back to
// the deterministic diagnosis unchanged -- PROBE never breaks the loop.
func (provider *GeminiProvider) Diagnose(ctx context.Context, evidence domain.Evidence, deterministic domain.Diagnosis) domain.Diagnosis {
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return deterministic
	}
	deterministicJSON, err := json.Marshal(deterministic)
	if err != nil {
		return deterministic
	}
	prompt := fmt.Sprintf(
		"You are explaining an electronics diagnostic finding to a hobbyist. "+
			"Use ONLY the evidence and deterministic finding below; never invent a "+
			"measurement, cause, or wire that is not implied by this evidence. "+
			"Rewrite the finding in clear, plain English. Confidence must stay "+
			"between 0 and 1 and must not exceed the deterministic confidence.\n\n"+
			"Evidence: %s\n\nDeterministic finding: %s",
		string(evidenceJSON), string(deterministicJSON),
	)
	requestBody := map[string]any{
		"contents": []any{map[string]any{"parts": []any{map[string]any{"text": prompt}}}},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"maxOutputTokens":  1024,
			"responseMimeType": "application/json",
			"responseSchema": map[string]any{
				"type": "OBJECT",
				"properties": map[string]any{
					"headline":        map[string]any{"type": "STRING"},
					"summary":         map[string]any{"type": "STRING"},
					"possible_causes": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
					"confidence":      map[string]any{"type": "NUMBER"},
					"next_test":       map[string]any{"type": "STRING"},
				},
				"required": []string{"headline", "summary", "possible_causes", "confidence", "next_test"},
			},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return deterministic
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(payload))
	if err != nil {
		return deterministic
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", provider.apiKey)
	response, err := provider.client.Do(req)
	if err != nil {
		return deterministic
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGeminiResponseBytes))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return deterministic
	}
	var envelope struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Candidates) == 0 || len(envelope.Candidates[0].Content.Parts) == 0 {
		return deterministic
	}
	var parsed geminiProbePayload
	if err := json.Unmarshal([]byte(envelope.Candidates[0].Content.Parts[0].Text), &parsed); err != nil {
		return deterministic
	}
	if strings.TrimSpace(parsed.Headline) == "" || strings.TrimSpace(parsed.Summary) == "" {
		return deterministic
	}
	if parsed.Confidence < 0 || parsed.Confidence > 1 || parsed.Confidence > deterministic.Confidence {
		parsed.Confidence = deterministic.Confidence
	}
	if strings.TrimSpace(parsed.NextTest) == "" {
		parsed.NextTest = deterministic.NextTest
	}
	return domain.Diagnosis{
		Headline:       parsed.Headline,
		Summary:        parsed.Summary,
		PossibleCauses: parsed.PossibleCauses,
		Confidence:     parsed.Confidence,
		NextTest:       parsed.NextTest,
	}
}
