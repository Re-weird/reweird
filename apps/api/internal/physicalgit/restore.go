package physicalgit

import (
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// Restore builds a deterministic restoration checklist for returning a
// project's OBSERVABLE state to target's captured state. It never
// physically modifies hardware and never invokes Gemini. It reuses this
// package's own comparison building blocks (diffComponentFields,
// diffConnectionFields, connectionSummary, visualDiff, softwareDiff,
// DiffVisionAnalyses) instead of a second, independent diff engine.
//
// source is the current/reference commit to compare target against, or nil
// when none is available/selected -- every section then reports UNAVAILABLE
// rather than fabricating a "current hardware state".
func Restore(target domain.PhysicalCommit, source *domain.PhysicalCommit, targetMeasurement, sourceMeasurement *domain.MeasurementWindow, targetVision, sourceVision *domain.PhysicalCommitVisionAnalysis) domain.PhysicalRestorePlan {
	plan := domain.PhysicalRestorePlan{
		ProjectID:    target.ProjectID,
		TargetCommit: target.ID,
		HasSource:    source != nil,
	}
	if source != nil {
		plan.SourceCommit = source.ID
	}

	targetLabel, sourceLabel := target.DisplayID, "the reference state"
	if source != nil {
		sourceLabel = source.DisplayID
	}

	if source == nil {
		unavailable := domain.RestoreSection{Status: domain.RestoreUnavailable, Actions: []domain.RestoreAction{{
			Status:      domain.RestoreUnavailable,
			Title:       "No reference commit selected",
			Description: fmt.Sprintf("Select a newer physical commit to compare against %s for restoration guidance.", targetLabel),
		}}}
		componentsSection, circuitSection, electricalSection, visualSection, softwareSection := unavailable, unavailable, unavailable, unavailable, unavailable
		componentsSection.Actions[0].Category = domain.RestoreCategoryComponents
		circuitSection.Actions[0].Category = domain.RestoreCategoryCircuit
		electricalSection.Actions[0].Category = domain.RestoreCategoryElectrical
		visualSection.Actions[0].Category = domain.RestoreCategoryVisual
		softwareSection.Actions[0].Category = domain.RestoreCategorySoftware
		plan.Components, plan.Circuit, plan.Electrical, plan.Visual, plan.Software = componentsSection, circuitSection, electricalSection, visualSection, softwareSection
		return plan
	}

	plan.Components = restoreComponents(target.ProfileSnapshot, source.ProfileSnapshot, targetLabel, sourceLabel)
	plan.Circuit = restoreCircuit(target.ProfileSnapshot, source.ProfileSnapshot, targetLabel, sourceLabel)
	plan.Electrical = restoreElectrical(target, *source, targetMeasurement, sourceMeasurement, targetLabel)
	plan.Visual = restoreVisual(target, *source, targetVision, sourceVision, targetLabel, sourceLabel)
	plan.Software = restoreSoftware(target, *source, targetLabel)
	return plan
}

func restoreComponents(target, source *domain.ProjectProfile, targetLabel, sourceLabel string) domain.RestoreSection {
	switch {
	case target == nil && source == nil:
		return domain.RestoreSection{Status: domain.RestoreNotCaptured}
	case target == nil || source == nil:
		return unavailableSection(domain.RestoreCategoryComponents, "One of the compared commits has no captured component snapshot.")
	}

	sourceByID := make(map[string]domain.ComponentSpecification, len(source.Components))
	for _, component := range source.Components {
		sourceByID[component.ID] = component
	}
	targetByID := make(map[string]domain.ComponentSpecification, len(target.Components))
	for _, component := range target.Components {
		targetByID[component.ID] = component
	}

	actions := make([]domain.RestoreAction, 0, len(target.Components))
	actionRequired := false
	for _, component := range target.Components {
		current, exists := sourceByID[component.ID]
		switch {
		case !exists:
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryComponents, Status: domain.RestoreActionRequired,
				Title: component.Name, TargetValue: component.Name, CurrentValue: "not present",
				Description: fmt.Sprintf("%s contains %s but %s does not.", targetLabel, component.Name, sourceLabel),
			})
			actionRequired = true
		case len(diffComponentFields(current, component)) > 0:
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryComponents, Status: domain.RestoreActionRequired,
				Title: component.Name, TargetValue: component.Name, CurrentValue: current.Name,
				Description: fmt.Sprintf("Restore %s to match %s's captured specification.", component.Name, targetLabel),
			})
			actionRequired = true
		default:
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryComponents, Status: domain.RestoreMatch,
				Title: component.Name, TargetValue: component.Name, CurrentValue: component.Name,
			})
		}
	}
	for _, component := range source.Components {
		if _, exists := targetByID[component.ID]; exists {
			continue
		}
		actions = append(actions, domain.RestoreAction{
			Category: domain.RestoreCategoryComponents, Status: domain.RestoreActionRequired,
			Title: component.Name, TargetValue: "not present", CurrentValue: component.Name,
			Description: fmt.Sprintf("%s does not contain %s.", targetLabel, component.Name),
		})
		actionRequired = true
	}

	status := domain.RestoreMatch
	if actionRequired {
		status = domain.RestoreActionRequired
	}
	return domain.RestoreSection{Status: status, Actions: actions}
}

func restoreCircuit(target, source *domain.ProjectProfile, targetLabel, sourceLabel string) domain.RestoreSection {
	switch {
	case target == nil && source == nil:
		return domain.RestoreSection{Status: domain.RestoreNotCaptured}
	case target == nil || source == nil:
		return unavailableSection(domain.RestoreCategoryCircuit, "One of the compared commits has no captured circuit snapshot.")
	}

	sourceByID := make(map[string]domain.ProfileConnection, len(source.Connections))
	for _, connection := range source.Connections {
		sourceByID[connection.ID] = connection
	}
	targetByID := make(map[string]domain.ProfileConnection, len(target.Connections))
	for _, connection := range target.Connections {
		targetByID[connection.ID] = connection
	}

	actions := make([]domain.RestoreAction, 0, len(target.Connections))
	actionRequired := false
	for _, connection := range target.Connections {
		current, exists := sourceByID[connection.ID]
		switch {
		case !exists:
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryCircuit, Status: domain.RestoreActionRequired,
				Title: connection.Role, TargetValue: connectionSummary(connection), CurrentValue: "not present",
				Description: fmt.Sprintf("%s contains %s but %s does not.", targetLabel, connectionSummary(connection), sourceLabel),
			})
			actionRequired = true
		case len(diffConnectionFields(current, connection)) > 0:
			description := fmt.Sprintf("Restore the %s connection to match %s.", connection.Role, targetLabel)
			if current.GPIO != nil && connection.GPIO != nil && *current.GPIO != *connection.GPIO {
				description = fmt.Sprintf("Restore %s connection to GPIO%d.", connection.Role, *connection.GPIO)
			}
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryCircuit, Status: domain.RestoreActionRequired,
				Title: connection.Role, TargetValue: connectionSummary(connection), CurrentValue: connectionSummary(current),
				Description: description,
			})
			actionRequired = true
		default:
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryCircuit, Status: domain.RestoreMatch,
				Title: connection.Role, TargetValue: connectionSummary(connection), CurrentValue: connectionSummary(connection),
			})
		}
	}
	for _, connection := range source.Connections {
		if _, exists := targetByID[connection.ID]; exists {
			continue
		}
		actions = append(actions, domain.RestoreAction{
			Category: domain.RestoreCategoryCircuit, Status: domain.RestoreActionRequired,
			Title: connection.Role, TargetValue: "not present", CurrentValue: connectionSummary(connection),
			Description: fmt.Sprintf("%s does not contain %s.", targetLabel, connectionSummary(connection)),
		})
		actionRequired = true
	}

	status := domain.RestoreMatch
	if actionRequired {
		status = domain.RestoreActionRequired
	}
	return domain.RestoreSection{Status: status, Actions: actions}
}

// restoreElectrical never asks for a structural action -- electrical
// behavior is not something a user can directly set, only re-measure. If
// target captured a measurement, this section is always VERIFY_REQUIRED
// (or UNAVAILABLE if the reference resolved but the target's did not).
func restoreElectrical(target, source domain.PhysicalCommit, targetMeasurement, sourceMeasurement *domain.MeasurementWindow, targetLabel string) domain.RestoreSection {
	switch {
	case target.MeasurementID == nil && source.MeasurementID == nil:
		return domain.RestoreSection{Status: domain.RestoreNotCaptured}
	case target.MeasurementID == nil || targetMeasurement == nil:
		return unavailableSection(domain.RestoreCategoryElectrical, fmt.Sprintf("%s has no resolvable captured measurement to restore toward.", targetLabel))
	}

	sourceByProbe := map[string]domain.DerivedFacts{}
	if sourceMeasurement != nil {
		for _, facts := range sourceMeasurement.Analysis.Probes {
			sourceByProbe[facts.Probe] = facts
		}
	}

	actions := make([]domain.RestoreAction, 0, len(targetMeasurement.Analysis.Probes))
	for _, facts := range targetMeasurement.Analysis.Probes {
		current, hasCurrent := sourceByProbe[facts.Probe]
		currentValue := "not measured in the reference state"
		if hasCurrent {
			currentValue = summarizeProbeFacts(current)
		}
		actions = append(actions, domain.RestoreAction{
			Category: domain.RestoreCategoryElectrical, Status: domain.RestoreVerifyRequired,
			Title: facts.Probe, TargetValue: summarizeProbeFacts(facts), CurrentValue: currentValue,
			Description: fmt.Sprintf("Re-measure %s after restoring physical connections; electrical behavior must be re-observed, not assumed.", facts.Probe),
		})
	}
	return domain.RestoreSection{Status: domain.RestoreVerifyRequired, Actions: actions}
}

func summarizeProbeFacts(facts domain.DerivedFacts) string {
	parts := make([]string, 0, 3)
	if facts.AverageVoltage != nil {
		parts = append(parts, fmt.Sprintf("%.2fV avg", *facts.AverageVoltage))
	}
	if facts.MissingExpectedActivity {
		parts = append(parts, "activity missing")
	} else {
		parts = append(parts, "activity present")
	}
	if facts.Stable {
		parts = append(parts, "stable")
	} else {
		parts = append(parts, "unstable")
	}
	summary := ""
	for index, part := range parts {
		if index > 0 {
			summary += ", "
		}
		summary += part
	}
	return summary
}

// restoreVisual never produces an ACTION_REQUIRED item -- neither raw image
// bytes nor AI-detected components are something a user can be told to
// directly "fix"; both only ever require re-observation/verification.
func restoreVisual(target, source domain.PhysicalCommit, targetVision, sourceVision *domain.PhysicalCommitVisionAnalysis, targetLabel, sourceLabel string) domain.RestoreSection {
	raw := visualDiff(target.Image, source.Image)
	actions := make([]domain.RestoreAction, 0, 2)
	switch raw.Status {
	case domain.EvidenceUnchanged:
		actions = append(actions, domain.RestoreAction{Category: domain.RestoreCategoryVisual, Status: domain.RestoreMatch, Title: "Raw image", Description: "Raw image evidence matches the target commit."})
	case domain.EvidenceChanged:
		actions = append(actions, domain.RestoreAction{Category: domain.RestoreCategoryVisual, Status: domain.RestoreVerifyRequired, Title: "Raw image", Description: fmt.Sprintf("Visual state differs. Use %s's saved image as a visual reference.", targetLabel)})
	case domain.EvidenceAdded, domain.EvidenceRemoved:
		actions = append(actions, domain.RestoreAction{Category: domain.RestoreCategoryVisual, Status: domain.RestoreUnavailable, Title: "Raw image", Description: fmt.Sprintf("Only one of %s/%s has a captured image.", targetLabel, sourceLabel)})
	}

	semantic := DiffVisionAnalyses(targetVision, sourceVision)
	switch semantic.Status {
	case domain.EvidenceUnavailable:
		actions = append(actions, domain.RestoreAction{Category: domain.RestoreCategoryVisual, Status: domain.RestoreUnavailable, Title: "AI-detected hardware", AIInterpreted: true, Description: "Only one commit has a persisted Gemini Vision analysis; analyze both to compare AI-detected components."})
	case domain.EvidenceUnchanged:
		actions = append(actions, domain.RestoreAction{Category: domain.RestoreCategoryVisual, Status: domain.RestoreMatch, Title: "AI-detected hardware", AIInterpreted: true, Description: "AI-detected components match (unconfirmed interpretation, not measured evidence)."})
	case domain.EvidenceChanged:
		for _, change := range semantic.Changes {
			description := fmt.Sprintf("AI detected %s count %d in %s vs %d in %s (unconfirmed interpretation).", change.Name, change.BeforeCount, targetLabel, change.AfterCount, sourceLabel)
			actions = append(actions, domain.RestoreAction{
				Category: domain.RestoreCategoryVisual, Status: domain.RestoreVerifyRequired, AIInterpreted: true,
				Title: change.Name, TargetValue: fmt.Sprintf("%d", change.BeforeCount), CurrentValue: fmt.Sprintf("%d", change.AfterCount),
				Description: description,
			})
		}
	}

	if raw.Status == domain.EvidenceNotCaptured && semantic.Status == domain.EvidenceNotCaptured {
		return domain.RestoreSection{Status: domain.RestoreNotCaptured}
	}
	status := domain.RestoreMatch
	for _, action := range actions {
		switch action.Status {
		case domain.RestoreVerifyRequired:
			status = domain.RestoreVerifyRequired
		case domain.RestoreUnavailable:
			if status == domain.RestoreMatch {
				status = domain.RestoreUnavailable
			}
		}
	}
	return domain.RestoreSection{Status: status, Actions: actions}
}

func restoreSoftware(target, source domain.PhysicalCommit, targetLabel string) domain.RestoreSection {
	diff := softwareDiff(source, target)
	switch diff.Status {
	case domain.EvidenceNotCaptured:
		return domain.RestoreSection{Status: domain.RestoreNotCaptured}
	case domain.EvidenceUnchanged:
		return domain.RestoreSection{Status: domain.RestoreMatch, Actions: []domain.RestoreAction{{
			Category: domain.RestoreCategorySoftware, Status: domain.RestoreMatch, Title: "Software revision", Description: "Software reference matches the target commit.",
		}}}
	}

	actions := make([]domain.RestoreAction, 0, len(diff.Fields))
	for _, field := range diff.Fields {
		actions = append(actions, domain.RestoreAction{
			Category: domain.RestoreCategorySoftware, Status: domain.RestoreActionRequired,
			Title: field.Field, TargetValue: formatFieldValue(field.After), CurrentValue: formatFieldValue(field.Before),
			Description: fmt.Sprintf("Manually align %s with %s's captured software reference (not automated; Physical Git never performs a Git checkout).", field.Field, targetLabel),
		})
	}
	return domain.RestoreSection{Status: domain.RestoreActionRequired, Actions: actions}
}

func formatFieldValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "not captured"
	case *string:
		if typed == nil {
			return "not captured"
		}
		return *typed
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func unavailableSection(category domain.RestoreActionCategory, description string) domain.RestoreSection {
	return domain.RestoreSection{Status: domain.RestoreUnavailable, Actions: []domain.RestoreAction{{
		Category: category, Status: domain.RestoreUnavailable, Title: "Evidence incomplete", Description: description,
	}}}
}
