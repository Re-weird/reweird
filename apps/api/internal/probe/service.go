package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

const maxProbeServiceResponseBytes = 1024 * 1024

// ServiceProvider sends main's authoritative StructuredEvidence contract to
// the Python intelligence service and accepts only main's existing Diagnosis
// shape in return. It fails closed to the deterministic diagnosis on every
// transport, status, parsing, or validation failure.
type ServiceProvider struct {
	endpoint string
	client   *http.Client
}

func NewService(baseURL string) Provider {
	if strings.TrimSpace(baseURL) == "" {
		return MockProvider{}
	}
	return &ServiceProvider{
		endpoint: strings.TrimRight(baseURL, "/") + "/probe/main-diagnosis",
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func NewServiceForTest(endpoint string, client *http.Client) *ServiceProvider {
	return &ServiceProvider{endpoint: endpoint, client: client}
}

func (provider *ServiceProvider) Diagnose(
	ctx context.Context,
	evidence domain.Evidence,
	deterministic domain.Diagnosis,
) domain.Diagnosis {
	payload, err := json.Marshal(struct {
		Evidence               domain.Evidence  `json:"evidence"`
		DeterministicDiagnosis domain.Diagnosis `json:"deterministic_diagnosis"`
	}{Evidence: evidence, DeterministicDiagnosis: deterministic})
	if err != nil {
		return deterministic
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(payload))
	if err != nil {
		return deterministic
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.client.Do(request)
	if err != nil {
		return deterministic
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return deterministic
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxProbeServiceResponseBytes+1))
	if err != nil || len(body) > maxProbeServiceResponseBytes {
		return deterministic
	}
	var diagnosis domain.Diagnosis
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&diagnosis); err != nil || !validServiceDiagnosis(diagnosis, deterministic) {
		return deterministic
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return deterministic
	}
	return diagnosis
}

func validServiceDiagnosis(diagnosis, deterministic domain.Diagnosis) bool {
	if strings.TrimSpace(diagnosis.Headline) == "" || len(diagnosis.Headline) > 200 ||
		strings.TrimSpace(diagnosis.Summary) == "" || len(diagnosis.Summary) > 2000 ||
		strings.TrimSpace(diagnosis.NextTest) == "" || len(diagnosis.NextTest) > 1000 ||
		diagnosis.Confidence < 0 || diagnosis.Confidence > 1 || diagnosis.Confidence > deterministic.Confidence ||
		len(diagnosis.PossibleCauses) > 10 {
		return false
	}
	for _, cause := range diagnosis.PossibleCauses {
		if strings.TrimSpace(cause) == "" || len(cause) > 500 {
			return false
		}
	}
	return true
}
