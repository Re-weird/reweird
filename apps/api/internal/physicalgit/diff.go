// Package physicalgit computes deterministic, structured comparisons
// between two Physical Commits. It never calls an LLM and never infers
// anything beyond what each commit's stored, structured evidence proves.
package physicalgit

import (
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// Diff compares two Physical Commits in the same project. fromMeasurement
// and toMeasurement, when non-nil, are the already-resolved MeasurementWindow
// rows referenced by from.MeasurementID / to.MeasurementID respectively.
func Diff(from, to domain.PhysicalCommit, fromMeasurement, toMeasurement *domain.MeasurementWindow) domain.PhysicalCommitDiff {
	return domain.PhysicalCommitDiff{
		ProjectID:  from.ProjectID,
		FromCommit: from.ID,
		ToCommit:   to.ID,
		Visual:     visualDiff(from.Image, to.Image),
		Components: componentsDiff(from.ProfileSnapshot, to.ProfileSnapshot),
		Circuit:    circuitDiff(from.ProfileSnapshot, to.ProfileSnapshot),
		Electrical: electricalDiff(from.MeasurementID, to.MeasurementID, fromMeasurement, toMeasurement),
		Software:   softwareDiff(from, to),
	}
}

func visualDiff(before, after *domain.ProjectMedia) domain.VisualDiff {
	diff := domain.VisualDiff{BeforeImage: before, AfterImage: after}
	switch {
	case before == nil && after == nil:
		diff.Status = domain.EvidenceNotCaptured
	case before == nil && after != nil:
		diff.Status = domain.EvidenceAdded
	case before != nil && after == nil:
		diff.Status = domain.EvidenceRemoved
	case before.SHA256 == after.SHA256:
		diff.Status = domain.EvidenceUnchanged
	default:
		diff.Status = domain.EvidenceChanged
	}
	return diff
}

func componentsDiff(before, after *domain.ProjectProfile) domain.ComponentsDiff {
	switch {
	case before == nil && after == nil:
		return domain.ComponentsDiff{Status: domain.EvidenceNotCaptured}
	case before == nil || after == nil:
		return domain.ComponentsDiff{Status: domain.EvidenceUnavailable}
	}

	beforeByID := make(map[string]domain.ComponentSpecification, len(before.Components))
	for _, component := range before.Components {
		beforeByID[component.ID] = component
	}
	afterByID := make(map[string]domain.ComponentSpecification, len(after.Components))
	for _, component := range after.Components {
		afterByID[component.ID] = component
	}

	changes := make([]domain.ComponentChange, 0)
	for _, component := range before.Components {
		if _, stillPresent := afterByID[component.ID]; stillPresent {
			continue
		}
		changes = append(changes, domain.ComponentChange{ComponentID: component.ID, Name: component.Name, Status: domain.EvidenceRemoved})
	}
	for _, component := range after.Components {
		previous, existed := beforeByID[component.ID]
		if !existed {
			changes = append(changes, domain.ComponentChange{ComponentID: component.ID, Name: component.Name, Status: domain.EvidenceAdded})
			continue
		}
		fields := diffComponentFields(previous, component)
		if len(fields) > 0 {
			changes = append(changes, domain.ComponentChange{ComponentID: component.ID, Name: component.Name, Status: domain.EvidenceChanged, Fields: fields})
		}
	}

	status := domain.EvidenceUnchanged
	if len(changes) > 0 {
		status = domain.EvidenceChanged
	}
	return domain.ComponentsDiff{Status: status, Changes: changes}
}

func diffComponentFields(before, after domain.ComponentSpecification) []domain.FieldChange {
	fields := make([]domain.FieldChange, 0)
	if before.Name != after.Name {
		fields = append(fields, domain.FieldChange{Field: "name", Before: before.Name, After: after.Name})
	}
	if before.Manufacturer != after.Manufacturer {
		fields = append(fields, domain.FieldChange{Field: "manufacturer", Before: before.Manufacturer, After: after.Manufacturer})
	}
	if before.InterfaceType != after.InterfaceType {
		fields = append(fields, domain.FieldChange{Field: "interface_type", Before: before.InterfaceType, After: after.InterfaceType})
	}
	if before.ExpectedBehavior != after.ExpectedBehavior {
		fields = append(fields, domain.FieldChange{Field: "expected_behavior", Before: before.ExpectedBehavior, After: after.ExpectedBehavior})
	}
	if before.Confirmed != after.Confirmed {
		fields = append(fields, domain.FieldChange{Field: "confirmed", Before: before.Confirmed, After: after.Confirmed})
	}
	return fields
}

func circuitDiff(before, after *domain.ProjectProfile) domain.CircuitDiff {
	switch {
	case before == nil && after == nil:
		return domain.CircuitDiff{Status: domain.EvidenceNotCaptured}
	case before == nil || after == nil:
		return domain.CircuitDiff{Status: domain.EvidenceUnavailable}
	}

	beforeByID := make(map[string]domain.ProfileConnection, len(before.Connections))
	for _, connection := range before.Connections {
		beforeByID[connection.ID] = connection
	}
	afterByID := make(map[string]domain.ProfileConnection, len(after.Connections))
	for _, connection := range after.Connections {
		afterByID[connection.ID] = connection
	}

	changes := make([]domain.ConnectionChange, 0)
	for _, connection := range before.Connections {
		if _, stillPresent := afterByID[connection.ID]; stillPresent {
			continue
		}
		changes = append(changes, domain.ConnectionChange{ConnectionID: connection.ID, Summary: connectionSummary(connection), Status: domain.EvidenceRemoved})
	}
	for _, connection := range after.Connections {
		previous, existed := beforeByID[connection.ID]
		if !existed {
			changes = append(changes, domain.ConnectionChange{ConnectionID: connection.ID, Summary: connectionSummary(connection), Status: domain.EvidenceAdded})
			continue
		}
		fields := diffConnectionFields(previous, connection)
		if len(fields) > 0 {
			changes = append(changes, domain.ConnectionChange{ConnectionID: connection.ID, Summary: connectionSummary(connection), Status: domain.EvidenceChanged, Fields: fields})
		}
	}

	status := domain.EvidenceUnchanged
	if len(changes) > 0 {
		status = domain.EvidenceChanged
	}
	return domain.CircuitDiff{Status: status, Changes: changes}
}

// connectionSummary is a deterministic label built only from stored fields,
// never inferred -- "GPIO13 -> SERVO" when a GPIO number was captured, else
// "ROLE -> target".
func connectionSummary(connection domain.ProfileConnection) string {
	if connection.GPIO != nil {
		return fmt.Sprintf("GPIO%d → %s", *connection.GPIO, connection.Target)
	}
	return fmt.Sprintf("%s → %s", connection.Role, connection.Target)
}

func diffConnectionFields(before, after domain.ProfileConnection) []domain.FieldChange {
	fields := make([]domain.FieldChange, 0)
	if before.ComponentID != after.ComponentID {
		fields = append(fields, domain.FieldChange{Field: "component_id", Before: before.ComponentID, After: after.ComponentID})
	}
	if before.ComponentName != after.ComponentName {
		fields = append(fields, domain.FieldChange{Field: "component_name", Before: before.ComponentName, After: after.ComponentName})
	}
	if before.Role != after.Role {
		fields = append(fields, domain.FieldChange{Field: "role", Before: before.Role, After: after.Role})
	}
	if !intPointerEqual(before.GPIO, after.GPIO) {
		fields = append(fields, domain.FieldChange{Field: "gpio", Before: before.GPIO, After: after.GPIO})
	}
	if before.Target != after.Target {
		fields = append(fields, domain.FieldChange{Field: "target", Before: before.Target, After: after.Target})
	}
	if before.Direction != after.Direction {
		fields = append(fields, domain.FieldChange{Field: "direction", Before: before.Direction, After: after.Direction})
	}
	if before.Behavior != after.Behavior {
		fields = append(fields, domain.FieldChange{Field: "behavior", Before: before.Behavior, After: after.Behavior})
	}
	if before.Required != after.Required {
		fields = append(fields, domain.FieldChange{Field: "required", Before: before.Required, After: after.Required})
	}
	if before.Confirmed != after.Confirmed {
		fields = append(fields, domain.FieldChange{Field: "confirmed", Before: before.Confirmed, After: after.Confirmed})
	}
	return fields
}

// electricalDiff never keys off MeasurementWindow.ID, CapturedAtMS,
// IngestedAtMS, or Sequence -- only the compared DerivedFacts fields decide
// UNCHANGED vs CHANGED, so two different measurement ids with identical
// compared content report UNCHANGED.
func electricalDiff(beforeID, afterID *int64, before, after *domain.MeasurementWindow) domain.ElectricalDiff {
	diff := domain.ElectricalDiff{BeforeMeasurementID: beforeID, AfterMeasurementID: afterID}
	switch {
	case beforeID == nil && afterID == nil:
		diff.Status = domain.EvidenceNotCaptured
		return diff
	case beforeID == nil && afterID != nil:
		diff.Status = domain.EvidenceAdded
		return diff
	case beforeID != nil && afterID == nil:
		diff.Status = domain.EvidenceRemoved
		return diff
	case before == nil || after == nil:
		// A MeasurementID was captured on both sides but at least one
		// failed to resolve -- never silently treat this as "not captured".
		diff.Status = domain.EvidenceUnavailable
		return diff
	}

	beforeByProbe := make(map[string]domain.DerivedFacts, len(before.Analysis.Probes))
	for _, facts := range before.Analysis.Probes {
		beforeByProbe[facts.Probe] = facts
	}
	afterByProbe := make(map[string]domain.DerivedFacts, len(after.Analysis.Probes))
	for _, facts := range after.Analysis.Probes {
		afterByProbe[facts.Probe] = facts
	}

	changes := make([]domain.ProbeElectricalChange, 0)
	for _, facts := range before.Analysis.Probes {
		if _, stillPresent := afterByProbe[facts.Probe]; stillPresent {
			continue
		}
		changes = append(changes, domain.ProbeElectricalChange{Probe: facts.Probe, Status: domain.EvidenceRemoved})
	}
	for _, facts := range after.Analysis.Probes {
		previous, existed := beforeByProbe[facts.Probe]
		if !existed {
			changes = append(changes, domain.ProbeElectricalChange{Probe: facts.Probe, Status: domain.EvidenceAdded})
			continue
		}
		fields := diffProbeFields(previous, facts)
		if len(fields) > 0 {
			changes = append(changes, domain.ProbeElectricalChange{Probe: facts.Probe, Status: domain.EvidenceChanged, Fields: fields})
		}
	}

	diff.Probes = changes
	diff.Status = domain.EvidenceUnchanged
	if len(changes) > 0 {
		diff.Status = domain.EvidenceChanged
	}
	return diff
}

func diffProbeFields(before, after domain.DerivedFacts) []domain.FieldChange {
	fields := make([]domain.FieldChange, 0)
	if !float64PointerEqual(before.AverageVoltage, after.AverageVoltage) {
		fields = append(fields, domain.FieldChange{Field: "average_voltage", Before: before.AverageVoltage, After: after.AverageVoltage})
	}
	if !float64PointerEqual(before.FrequencyHz, after.FrequencyHz) {
		fields = append(fields, domain.FieldChange{Field: "frequency_hz", Before: before.FrequencyHz, After: after.FrequencyHz})
	}
	if before.MissingExpectedActivity != after.MissingExpectedActivity {
		fields = append(fields, domain.FieldChange{Field: "missing_expected_activity", Before: before.MissingExpectedActivity, After: after.MissingExpectedActivity})
	}
	if before.Stable != after.Stable {
		fields = append(fields, domain.FieldChange{Field: "stable", Before: before.Stable, After: after.Stable})
	}
	if before.DropoutEvents != after.DropoutEvents {
		fields = append(fields, domain.FieldChange{Field: "dropout_events", Before: before.DropoutEvents, After: after.DropoutEvents})
	}
	if !intPointerEqual(before.DigitalState, after.DigitalState) {
		fields = append(fields, domain.FieldChange{Field: "digital_state", Before: before.DigitalState, After: after.DigitalState})
	}
	return fields
}

func softwareDiff(from, to domain.PhysicalCommit) domain.SoftwareDiff {
	if from.SoftwareProvider == nil && to.SoftwareProvider == nil &&
		from.SoftwareRepository == nil && to.SoftwareRepository == nil &&
		from.SoftwareRevision == nil && to.SoftwareRevision == nil {
		return domain.SoftwareDiff{Status: domain.EvidenceNotCaptured}
	}

	fields := make([]domain.FieldChange, 0)
	if !stringPointerEqual(from.SoftwareProvider, to.SoftwareProvider) {
		fields = append(fields, domain.FieldChange{Field: "software_provider", Before: from.SoftwareProvider, After: to.SoftwareProvider})
	}
	if !stringPointerEqual(from.SoftwareRepository, to.SoftwareRepository) {
		fields = append(fields, domain.FieldChange{Field: "software_repository", Before: from.SoftwareRepository, After: to.SoftwareRepository})
	}
	if !stringPointerEqual(from.SoftwareRevision, to.SoftwareRevision) {
		fields = append(fields, domain.FieldChange{Field: "software_revision", Before: from.SoftwareRevision, After: to.SoftwareRevision})
	}

	status := domain.EvidenceUnchanged
	if len(fields) > 0 {
		status = domain.EvidenceChanged
	}
	return domain.SoftwareDiff{Status: status, Fields: fields}
}

func intPointerEqual(before, after *int) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}

func float64PointerEqual(before, after *float64) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}

func stringPointerEqual(before, after *string) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}
