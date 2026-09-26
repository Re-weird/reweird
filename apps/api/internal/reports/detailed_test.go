package reports

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestDetailedReportRedactsSecretsAndNeverIncludesSourceCode(t *testing.T) {
	workflow := domain.DiagnosticWorkflow{
		ID: "test-1234567890abcdef1234567890abcdef", SessionID: "session-generic", ProjectID: "project", ProfileID: "generic", ProfileVersion: 2,
		CreatedAtMS: 12345, UpdatedAtMS: 23456, Status: domain.TestCompleted,
		ProfileSnapshot: &domain.ProjectProfile{ID: "generic", ProjectName: "Servo Controller", Controller: "ESP32", ExpectedBehavior: "Send pulses; GEMINI_API_KEY=topsecretvalue123"},
		Plan:            domain.TestPlan{Title: "Timing test", Recommendation: domain.TestRecommendation{Reason: "Observed jitter; Bearer abcdefghijklmnop", TestType: domain.TestFrequencyTiming}},
		UserActions:     []domain.UserAction{{ID: "action", Description: "restarted; ghp_123456789012345678901234567890", TimestampMS: 20000}},
	}
	report, err := BuildDetailed(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if report.SecurityRedactionCount < 3 {
		t.Fatalf("redactions = %d", report.SecurityRedactionCount)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	markdown := Markdown(report)
	for _, secret := range []string{"topsecretvalue123", "abcdefghijklmnop", "ghp_123456789012345678901234567890"} {
		if strings.Contains(string(encoded), secret) || strings.Contains(markdown, secret) {
			t.Fatalf("secret %q leaked", secret)
		}
	}
	if !strings.Contains(markdown, "\\[REDACTED\\_SECRET\\]") {
		t.Fatalf("redaction not visible in Markdown: %q", markdown)
	}
	if strings.Contains(string(encoded), "code_text") || strings.Contains(markdown, "main.ino") {
		t.Fatal("uploaded source leaked")
	}
}

func TestDetailedReportDeterministic(t *testing.T) {
	workflow := domain.DiagnosticWorkflow{ID: "test-1234567890abcdef1234567890abcdef", CreatedAtMS: 1, UpdatedAtMS: 2, Status: domain.TestPlanned, Plan: domain.TestPlan{Title: "Signal activity", Recommendation: domain.TestRecommendation{Reason: "No activity"}}}
	first, err := BuildDetailed(workflow)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDetailed(workflow)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := JSON(first)
	b, _ := JSON(second)
	if string(a) != string(b) {
		t.Fatalf("report is nondeterministic")
	}
}

func TestConfiguredEnvironmentSecretIsRedacted(t *testing.T) {
	t.Setenv("REWEIRD_TEST_SECRET", "opaque-value-without-a-pattern")
	clean, count := SanitizeText("user note: opaque-value-without-a-pattern")
	if count != 1 || strings.Contains(clean, "opaque-value-without-a-pattern") {
		t.Fatalf("environment secret escaped redaction: %q (%d)", clean, count)
	}
}
