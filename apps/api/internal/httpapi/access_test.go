package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIBearerAccessControl(t *testing.T) {
	t.Setenv("REWEIRD_API_TOKEN", "test-api-token-with-at-least-32-characters")
	app, _ := testApp(t)

	for _, test := range []struct {
		name   string
		header string
		want   int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer wrong", http.StatusUnauthorized},
		{"query-token-is-not-auth", "", http.StatusUnauthorized},
		{"valid", "Bearer test-api-token-with-at-least-32-characters", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := "/api/v1/status"
			if test.name == "query-token-is-not-auth" {
				path += "?token=test-api-token-with-at-least-32-characters"
			}
			request := httptest.NewRequest(http.MethodGet, path, nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
		})
	}

	health, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", health.StatusCode)
	}

	patchRequest := httptest.NewRequest(http.MethodPost, "/api/v1/patch", nil)
	patchRequest.Header.Set("Authorization", "Bearer test-api-token-with-at-least-32-characters")
	patch, err := app.Test(patchRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusLocked {
		t.Fatalf("authenticated PATCH status = %d, want locked", patch.StatusCode)
	}
}
