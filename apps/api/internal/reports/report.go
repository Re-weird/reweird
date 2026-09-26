// Package reports turns a diagnostic Session into a persistable Report,
// including the VERIFY result (fixed / still_failing / unclear).
package reports

import "github.com/re-weird/reweird/apps/api/internal/domain"

func BuildReport(session domain.Session) domain.Report {
	report := domain.Report{
		SessionID:      session.ID,
		ProjectName:    session.ProjectName,
		ProfileID:      session.ProfileID,
		Stage:          session.Stage,
		Headline:       session.Diagnosis.Headline,
		Summary:        session.Diagnosis.Summary,
		PossibleCauses: session.Diagnosis.PossibleCauses,
		Confidence:     session.Diagnosis.Confidence,
		NextTest:       session.Diagnosis.NextTest,
		Before:         session.Before,
		After:          session.After,
		Evidence:       session.Evidence,
	}

	if session.Stage == domain.StageVerify {
		result := verifyResult(session)
		report.Verify = &result
	}

	return report
}

func verifyResult(session domain.Session) domain.VerifyResult {
	if session.After == nil {
		return domain.VerifyUnclear
	}
	if hasFailure(session.Evidence.RuleResults) {
		return domain.VerifyStillFailing
	}
	if session.After.Stability == "stable" {
		return domain.VerifyFixed
	}
	return domain.VerifyStillFailing
}

func hasFailure(results []domain.RuleResult) bool {
	for _, result := range results {
		if result.Status == "fail" {
			return true
		}
	}
	return false
}
