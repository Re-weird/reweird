package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

const maxGeminiImageBytes = 5 * 1024 * 1024

type Analyzer interface {
	Analyze(ctx context.Context, imagePath, contentType string) domain.VisionAnalysis
}

type SkippedAnalyzer struct{}

func (SkippedAnalyzer) Analyze(context.Context, string, string) domain.VisionAnalysis {
	return domain.VisionAnalysis{
		Status:        "VISION_SKIPPED",
		Components:    []domain.VisionComponent{},
		Relationships: []domain.VisionRelationship{},
		Warnings:      []string{"Gemini Vision was skipped because GEMINI_API_KEY is not configured."},
	}
}

type GeminiAnalyzer struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

func NewGemini(apiKey, model string) Analyzer {
	if strings.TrimSpace(apiKey) == "" {
		return SkippedAnalyzer{}
	}
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiAnalyzer{
		apiKey:   apiKey,
		model:    model,
		endpoint: "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent",
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func NewGeminiForTest(apiKey, model, endpoint string, client *http.Client) *GeminiAnalyzer {
	return &GeminiAnalyzer{apiKey: apiKey, model: model, endpoint: endpoint, client: client}
}

type geminiVisionPayload struct {
	Components []struct {
		CatalogID     string   `json:"catalog_id"`
		Name          string   `json:"name"`
		Confidence    float64  `json:"confidence"`
		VisibleLabels []string `json:"visible_labels"`
	} `json:"components"`
	Relationships []struct {
		From       string  `json:"from"`
		To         string  `json:"to"`
		Role       string  `json:"role"`
		GPIO       *int    `json:"gpio"`
		Confidence float64 `json:"confidence"`
	} `json:"relationships"`
	Warnings []string `json:"warnings"`
}

func (analyzer *GeminiAnalyzer) Analyze(ctx context.Context, imagePath, contentType string) domain.VisionAnalysis {
	result := domain.VisionAnalysis{Status: "VISION_FAILED", Model: analyzer.model, Components: []domain.VisionComponent{}, Relationships: []domain.VisionRelationship{}, Warnings: []string{}}
	image, err := readBounded(imagePath, maxGeminiImageBytes)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		return result
	}
	requestBody := map[string]any{
		"contents": []any{map[string]any{"parts": []any{
			map[string]any{"text": "Identify only visible electronics components and printed labels in this project photo. Suggest possible relationships only when visually supported. Never state wiring as confirmed fact. Use catalog IDs when one of these matches: hc-sr04, sg90-servo, led, push-button, generic-digital-input, generic-digital-output, pwm-output, i2c-device, uart-device. Confidence must be between 0 and 1."},
			map[string]any{"inlineData": map[string]any{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(image)}},
		}}},
		"generationConfig": map[string]any{
			"temperature":      0.1,
			"maxOutputTokens":  2048,
			"responseMimeType": "application/json",
			"responseSchema": map[string]any{
				"type": "OBJECT",
				"properties": map[string]any{
					"components":    map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{"catalog_id": map[string]any{"type": "STRING"}, "name": map[string]any{"type": "STRING"}, "confidence": map[string]any{"type": "NUMBER"}, "visible_labels": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}}}, "required": []string{"catalog_id", "name", "confidence", "visible_labels"}}},
					"relationships": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{"from": map[string]any{"type": "STRING"}, "to": map[string]any{"type": "STRING"}, "role": map[string]any{"type": "STRING"}, "gpio": map[string]any{"type": "INTEGER", "nullable": true}, "confidence": map[string]any{"type": "NUMBER"}}, "required": []string{"from", "to", "role", "confidence"}}},
					"warnings":      map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
				},
				"required": []string{"components", "relationships", "warnings"},
			},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		result.Warnings = append(result.Warnings, "encode Gemini request: "+err.Error())
		return result
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, analyzer.endpoint, bytes.NewReader(payload))
	if err != nil {
		result.Warnings = append(result.Warnings, "create Gemini request: "+err.Error())
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", analyzer.apiKey)
	response, err := analyzer.client.Do(req)
	if err != nil {
		result.Warnings = append(result.Warnings, "Gemini request failed: "+err.Error())
		return result
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		result.Warnings = append(result.Warnings, "read Gemini response: "+err.Error())
		return result
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Gemini returned HTTP %d", response.StatusCode))
		return result
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
	if err := json.Unmarshal(responseBody, &envelope); err != nil || len(envelope.Candidates) == 0 || len(envelope.Candidates[0].Content.Parts) == 0 {
		result.Warnings = append(result.Warnings, "Gemini returned no structured vision result.")
		return result
	}
	var parsed geminiVisionPayload
	if err := json.Unmarshal([]byte(envelope.Candidates[0].Content.Parts[0].Text), &parsed); err != nil {
		result.Warnings = append(result.Warnings, "Gemini vision JSON failed validation.")
		return result
	}
	for _, component := range parsed.Components {
		if component.Name == "" || component.Confidence < 0 || component.Confidence > 1 {
			continue
		}
		result.Components = append(result.Components, domain.VisionComponent{CatalogID: component.CatalogID, Name: component.Name, Confidence: component.Confidence, VisibleLabels: component.VisibleLabels, Source: domain.SourceVisionAI})
	}
	for _, relationship := range parsed.Relationships {
		if relationship.From == "" || relationship.To == "" || relationship.Confidence < 0 || relationship.Confidence > 1 {
			continue
		}
		result.Relationships = append(result.Relationships, domain.VisionRelationship{From: relationship.From, To: relationship.To, Role: relationship.Role, GPIO: relationship.GPIO, Confidence: relationship.Confidence, Source: domain.SourceVisionAI})
	}
	result.Status = "VISION_COMPLETE"
	result.Warnings = append(result.Warnings, parsed.Warnings...)
	result.Warnings = append(result.Warnings, "Vision findings are AI suggestions and require user confirmation.")
	return result
}

func readBounded(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open uploaded image: %w", err)
	}
	defer file.Close()
	reader := io.LimitReader(file, maximum+1)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read uploaded image: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("uploaded image is empty")
	}
	if int64(len(payload)) > maximum {
		return nil, errors.New("uploaded image exceeds the Gemini input limit")
	}
	return payload, nil
}
