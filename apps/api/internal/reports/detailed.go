package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/history"
)

const MaxDetailedReportBytes = 1 << 20

type Fact struct {
	Probe      string            `json:"probe,omitempty"`
	WindowID   int64             `json:"window_id,omitempty"`
	Metric     string            `json:"metric"`
	Value      any               `json:"value"`
	Unit       string            `json:"unit,omitempty"`
	Provenance domain.Provenance `json:"provenance"`
}

type DetailedReport struct {
	ReportID               string                     `json:"report_id"`
	ProjectID              string                     `json:"project_id"`
	ProjectName            string                     `json:"project_name"`
	SessionID              string                     `json:"session_id"`
	DateMS                 int64                      `json:"date_ms"`
	Controller             string                     `json:"controller,omitempty"`
	ProfileID              string                     `json:"profile_id"`
	ProfileRevision        int                        `json:"profile_revision"`
	Summary                string                     `json:"summary"`
	ObservedBehavior       string                     `json:"observed_behavior"`
	ExpectedBehavior       string                     `json:"expected_behavior,omitempty"`
	MeasuredEvidence       []Fact                     `json:"measured_evidence"`
	DerivedEvidence        []Fact                     `json:"derived_evidence"`
	TestsPerformed         []domain.TestPlan          `json:"tests_performed"`
	TestResults            []domain.TestResult        `json:"test_results"`
	UserActions            []domain.UserAction        `json:"user_actions"`
	BeforeWindowID         int64                      `json:"before_window_id,omitempty"`
	AfterWindowID          int64                      `json:"after_window_id,omitempty"`
	VerifyResult           *domain.VerificationResult `json:"verify_result,omitempty"`
	FinalStatus            domain.HistoryStatus       `json:"final_status"`
	UnresolvedItems        []string                   `json:"unresolved_items"`
	Provenance             []domain.Provenance        `json:"provenance"`
	SystemInformation      map[string]any             `json:"system_information"`
	SecurityRedactionCount int                        `json:"security_redaction_count"`
}

// BuildDetailed creates a deterministic report only from persisted workflow
// evidence. It neither calls Gemini nor treats interpretation as measurement.
func BuildDetailed(workflow domain.DiagnosticWorkflow) (DetailedReport, error) {
	if workflow.ID == "" || workflow.CreatedAtMS == 0 {
		return DetailedReport{}, errors.New("persisted workflow is required")
	}
	summary := history.Summary(workflow)
	report := DetailedReport{
		ReportID: "report-" + workflow.ID, ProjectID: workflow.ProjectID, ProjectName: summary.ProjectName,
		SessionID: workflow.SessionID, DateMS: workflow.CreatedAtMS, ProfileID: workflow.ProfileID,
		ProfileRevision: workflow.ProfileVersion, Summary: "Structured evidence for " + workflow.Plan.Title + ".",
		ObservedBehavior: workflow.Plan.Recommendation.Reason,
		MeasuredEvidence: []Fact{}, DerivedEvidence: []Fact{}, TestsPerformed: []domain.TestPlan{workflow.Plan},
		TestResults: []domain.TestResult{}, UserActions: append([]domain.UserAction(nil), workflow.UserActions...),
		FinalStatus: summary.Status, UnresolvedItems: []string{},
		Provenance:        []domain.Provenance{domain.ProvenanceMeasured, domain.ProvenanceDerived, domain.ProvenanceProjectProfile, domain.ProvenanceGuidedTest},
		SystemInformation: map[string]any{"telemetry_source": summary.TelemetrySource},
	}
	if workflow.ProfileSnapshot != nil {
		report.Controller = workflow.ProfileSnapshot.Controller
		report.ExpectedBehavior = workflow.ProfileSnapshot.ExpectedBehavior
	}
	if workflow.Baseline != nil {
		report.BeforeWindowID = workflow.Baseline.ID
	}
	if workflow.After != nil {
		report.AfterWindowID = workflow.After.ID
	}
	for _, window := range []*domain.MeasurementWindow{workflow.Baseline, workflow.During, workflow.After} {
		if window == nil {
			continue
		}
		report.SystemInformation["telemetry_schema_version"] = window.Raw.SchemaVersion
		report.MeasuredEvidence = append(report.MeasuredEvidence, Fact{WindowID: window.ID, Metric: "capture_time_ms", Value: window.CapturedAtMS, Unit: "ms", Provenance: domain.ProvenanceMeasured})
		for _, sample := range window.Raw.Samples {
			report.MeasuredEvidence = append(report.MeasuredEvidence, Fact{Probe: sample.Probe, WindowID: window.ID, Metric: "raw_sample_count", Value: len(sample.AnalogMV) + len(sample.PeriodsUS) + len(sample.ActivityCounts), Unit: "samples", Provenance: domain.ProvenanceMeasured})
		}
		for _, facts := range window.Analysis.Probes {
			appendDerived := func(metric string, value any, unit string) {
				report.DerivedEvidence = append(report.DerivedEvidence, Fact{Probe: facts.Probe, WindowID: window.ID, Metric: metric, Value: value, Unit: unit, Provenance: domain.ProvenanceDerived})
			}
			appendDerived("dropout_events", facts.DropoutEvents, "events")
			appendDerived("stable", facts.Stable, "")
			appendDerived("missing_expected_activity", facts.MissingExpectedActivity, "")
			if facts.AverageVoltage != nil {
				appendDerived("average_voltage", *facts.AverageVoltage, "V")
			}
			if facts.MinimumVoltage != nil {
				appendDerived("minimum_voltage", *facts.MinimumVoltage, "V")
			}
			if facts.MaximumVoltage != nil {
				appendDerived("maximum_voltage", *facts.MaximumVoltage, "V")
			}
			if facts.FrequencyHz != nil {
				appendDerived("frequency", *facts.FrequencyHz, "Hz")
			}
			if facts.JitterUS != nil {
				appendDerived("jitter", *facts.JitterUS, "us")
			}
			if facts.BaselineDeviationPercent != nil {
				report.DerivedEvidence = append(report.DerivedEvidence, Fact{Probe: facts.Probe, WindowID: window.ID, Metric: "trusted_baseline_deviation", Value: *facts.BaselineDeviationPercent, Unit: "%", Provenance: domain.ProvenanceBaseline})
			}
		}
	}
	if workflow.Result != nil {
		report.TestResults = append(report.TestResults, *workflow.Result)
	}
	if workflow.Verification != nil {
		copyOfVerification := *workflow.Verification
		report.VerifyResult = &copyOfVerification
		report.UnresolvedItems = append(report.UnresolvedItems, workflow.Verification.RemainingIssues...)
	} else {
		report.UnresolvedItems = append(report.UnresolvedItems, "VERIFY has not been completed.")
	}
	if len(workflow.UserActions) > 0 {
		report.Provenance = append(report.Provenance, domain.ProvenanceUser)
	}
	if workflow.ProfileSnapshot != nil {
		for _, probe := range workflow.ProfileSnapshot.Probes {
			if probe.Baseline != nil && probe.Baseline.Status.Trusted() {
				report.Provenance = append(report.Provenance, domain.ProvenanceBaseline)
				break
			}
		}
	}
	sanitized, count, err := Sanitize(report)
	if err != nil {
		return DetailedReport{}, err
	}
	sanitized.SecurityRedactionCount = count
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return DetailedReport{}, err
	}
	if len(encoded) > MaxDetailedReportBytes {
		return DetailedReport{}, fmt.Errorf("report exceeds %d-byte export limit", MaxDetailedReportBytes)
	}
	return sanitized, nil
}

func JSON(report DetailedReport) ([]byte, error) { return json.MarshalIndent(report, "", "  ") }

func Markdown(report DetailedReport) string {
	var output strings.Builder
	line := func(value string) { output.WriteString(value); output.WriteByte('\n') }
	text := func(value string) string { return escapeMarkdown(value) }
	line("# ReWeird diagnostic report")
	line("")
	line("Report ID: " + text(report.ReportID))
	line("Project: " + text(report.ProjectName))
	line("Session: " + text(report.SessionID))
	line(fmt.Sprintf("Project Profile: %s · revision %d", text(report.ProfileID), report.ProfileRevision))
	line("Controller: " + text(report.Controller))
	line("Final status: " + string(report.FinalStatus))
	line("")
	for _, section := range []struct{ heading, content string }{{"Summary", report.Summary}, {"Observed behavior", report.ObservedBehavior}, {"Expected behavior", report.ExpectedBehavior}} {
		line("## " + section.heading)
		line(text(section.content))
		line("")
	}
	writeFacts := func(heading string, facts []Fact) {
		line("## " + heading)
		if len(facts) == 0 {
			line("No captured facts.")
		}
		for _, fact := range facts {
			line(fmt.Sprintf("- %s %s: %s %s (window #%d; %s)", text(fact.Probe), text(fact.Metric), text(fmt.Sprint(fact.Value)), text(fact.Unit), fact.WindowID, fact.Provenance))
		}
		line("")
	}
	writeFacts("Measured evidence", report.MeasuredEvidence)
	writeFacts("Derived evidence", report.DerivedEvidence)
	line("## Tests performed")
	for _, plan := range report.TestsPerformed {
		line("- " + text(plan.Title) + ": " + text(plan.Criteria))
	}
	line("")
	line("## Test results")
	for _, result := range report.TestResults {
		line("- " + text(result.Result) + ": " + text(result.Interpretation))
	}
	line("")
	line("## User actions")
	for _, action := range report.UserActions {
		line("- " + text(action.Description))
	}
	line("")
	line(fmt.Sprintf("## Before / after\nBefore window: #%d; after window: #%d", report.BeforeWindowID, report.AfterWindowID))
	line("")
	line("## VERIFY result")
	if report.VerifyResult == nil {
		line("Not completed.")
	} else {
		line(text(report.VerifyResult.Status) + ": " + text(report.VerifyResult.Summary))
	}
	line("")
	line("## Unresolved items")
	for _, item := range report.UnresolvedItems {
		line("- " + text(item))
	}
	line("")
	line("## Provenance")
	for _, item := range report.Provenance {
		line("- " + string(item))
	}
	line("")
	line("## System information")
	line("Telemetry source: " + text(fmt.Sprint(report.SystemInformation["telemetry_source"])))
	if report.SecurityRedactionCount > 0 {
		line(fmt.Sprintf("Security: %d potential secrets redacted before export.", report.SecurityRedactionCount))
	}
	return output.String()
}

func escapeMarkdown(value string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`", "<", "&lt;", ">", "&gt;").Replace(value)
}
