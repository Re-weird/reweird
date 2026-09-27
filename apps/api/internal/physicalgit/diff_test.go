package physicalgit

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func float64Ptr(value float64) *float64 { return &value }
func intPtr(value int) *int             { return &value }
func int64Ptr(value int64) *int64       { return &value }
func stringPtr(value string) *string    { return &value }

func TestDiffIdenticalSnapshotsAreUnchangedOrNotCaptured(t *testing.T) {
	commit := domain.PhysicalCommit{ID: "pcommit-a", ProjectID: "project-a"}
	diff := Diff(commit, commit, nil, nil, nil, nil)
	if diff.Visual.Status != domain.EvidenceNotCaptured {
		t.Fatalf("visual = %s", diff.Visual.Status)
	}
	if diff.Components.Status != domain.EvidenceNotCaptured {
		t.Fatalf("components = %s", diff.Components.Status)
	}
	if diff.Circuit.Status != domain.EvidenceNotCaptured {
		t.Fatalf("circuit = %s", diff.Circuit.Status)
	}
	if diff.Electrical.Status != domain.EvidenceNotCaptured {
		t.Fatalf("electrical = %s", diff.Electrical.Status)
	}
	if diff.Software.Status != domain.EvidenceNotCaptured {
		t.Fatalf("software = %s", diff.Software.Status)
	}
}

func TestDiffVisualStates(t *testing.T) {
	image := &domain.ProjectMedia{SHA256: "abc"}
	sameImage := &domain.ProjectMedia{SHA256: "abc"}
	otherImage := &domain.ProjectMedia{SHA256: "def"}

	if got := visualDiff(nil, nil).Status; got != domain.EvidenceNotCaptured {
		t.Fatalf("both missing = %s", got)
	}
	if got := visualDiff(nil, image).Status; got != domain.EvidenceAdded {
		t.Fatalf("added = %s", got)
	}
	if got := visualDiff(image, nil).Status; got != domain.EvidenceRemoved {
		t.Fatalf("removed = %s", got)
	}
	if got := visualDiff(image, sameImage).Status; got != domain.EvidenceUnchanged {
		t.Fatalf("same sha = %s", got)
	}
	if got := visualDiff(image, otherImage).Status; got != domain.EvidenceChanged {
		t.Fatalf("different sha = %s", got)
	}
}

func TestDiffComponentsAddedRemovedChanged(t *testing.T) {
	before := &domain.ProjectProfile{Components: []domain.ComponentSpecification{
		{ID: "c1", Name: "Ultrasonic sensor", Confirmed: false},
		{ID: "c2", Name: "Buzzer"},
	}}
	after := &domain.ProjectProfile{Components: []domain.ComponentSpecification{
		{ID: "c1", Name: "Ultrasonic sensor", Confirmed: true},
		{ID: "c3", Name: "SG90 Servo"},
	}}

	diff := componentsDiff(before, after)
	if diff.Status != domain.EvidenceChanged {
		t.Fatalf("status = %s", diff.Status)
	}
	byID := map[string]domain.ComponentChange{}
	for _, change := range diff.Changes {
		byID[change.ComponentID] = change
	}
	if byID["c2"].Status != domain.EvidenceRemoved {
		t.Fatalf("c2 = %#v", byID["c2"])
	}
	if byID["c3"].Status != domain.EvidenceAdded {
		t.Fatalf("c3 = %#v", byID["c3"])
	}
	if byID["c1"].Status != domain.EvidenceChanged || len(byID["c1"].Fields) != 1 || byID["c1"].Fields[0].Field != "confirmed" {
		t.Fatalf("c1 = %#v", byID["c1"])
	}
}

func TestDiffComponentsUnavailableWhenOneSideHasNoProfile(t *testing.T) {
	after := &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "Servo"}}}
	if got := componentsDiff(nil, after).Status; got != domain.EvidenceUnavailable {
		t.Fatalf("status = %s", got)
	}
	if got := circuitDiff(after, nil).Status; got != domain.EvidenceUnavailable {
		t.Fatalf("status = %s", got)
	}
}

func TestDiffCircuitConnectionAddedRemovedChangedAndSummary(t *testing.T) {
	before := &domain.ProjectProfile{Connections: []domain.ProfileConnection{
		{ID: "conn1", Role: "ECHO", GPIO: intPtr(18), Target: "ECHO"},
	}}
	after := &domain.ProjectProfile{Connections: []domain.ProfileConnection{
		{ID: "conn2", Role: "SIGNAL", GPIO: intPtr(13), Target: "SERVO"},
	}}

	diff := circuitDiff(before, after)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 2 {
		t.Fatalf("diff = %#v", diff)
	}
	byID := map[string]domain.ConnectionChange{}
	for _, change := range diff.Changes {
		byID[change.ConnectionID] = change
	}
	if byID["conn1"].Status != domain.EvidenceRemoved || byID["conn1"].Summary != "GPIO18 → ECHO" {
		t.Fatalf("conn1 = %#v", byID["conn1"])
	}
	if byID["conn2"].Status != domain.EvidenceAdded || byID["conn2"].Summary != "GPIO13 → SERVO" {
		t.Fatalf("conn2 = %#v", byID["conn2"])
	}
}

// TestDiffElectricalNeverKeysOffMeasurementIdentity is the required "ID
// trap" test: two MeasurementWindows with different IDs/timestamps but
// identical compared DerivedFacts content must diff as UNCHANGED.
func TestDiffElectricalNeverKeysOffMeasurementIdentity(t *testing.T) {
	before := &domain.MeasurementWindow{
		ID: 41, CapturedAtMS: 1000, IngestedAtMS: 1000,
		Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{
			{Probe: "ECHO", AverageVoltage: float64Ptr(3.3), Stable: true},
		}},
	}
	after := &domain.MeasurementWindow{
		ID: 99, CapturedAtMS: 9999, IngestedAtMS: 9999,
		Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{
			{Probe: "ECHO", AverageVoltage: float64Ptr(3.3), Stable: true},
		}},
	}
	diff := electricalDiff(int64Ptr(41), int64Ptr(99), before, after)
	if diff.Status != domain.EvidenceUnchanged {
		t.Fatalf("status = %s, want UNCHANGED despite different measurement ids", diff.Status)
	}
}

func TestDiffElectricalDetectsRealContentChange(t *testing.T) {
	before := &domain.MeasurementWindow{ID: 1, Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{
		{Probe: "ECHO", MissingExpectedActivity: false},
		{Probe: "POWER", AverageVoltage: float64Ptr(3.31)},
	}}}
	after := &domain.MeasurementWindow{ID: 2, Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{
		{Probe: "ECHO", MissingExpectedActivity: true},
		{Probe: "POWER", AverageVoltage: float64Ptr(3.29)},
	}}}
	diff := electricalDiff(int64Ptr(1), int64Ptr(2), before, after)
	if diff.Status != domain.EvidenceChanged || len(diff.Probes) != 2 {
		t.Fatalf("diff = %#v", diff)
	}
	byProbe := map[string]domain.ProbeElectricalChange{}
	for _, change := range diff.Probes {
		byProbe[change.Probe] = change
	}
	if byProbe["ECHO"].Fields[0].Field != "missing_expected_activity" {
		t.Fatalf("echo = %#v", byProbe["ECHO"])
	}
	if byProbe["POWER"].Fields[0].Field != "average_voltage" {
		t.Fatalf("power = %#v", byProbe["POWER"])
	}
}

func TestDiffElectricalMeasurementMissingOnOneSide(t *testing.T) {
	if got := electricalDiff(nil, nil, nil, nil).Status; got != domain.EvidenceNotCaptured {
		t.Fatalf("both nil = %s", got)
	}
	if got := electricalDiff(nil, int64Ptr(1), nil, &domain.MeasurementWindow{}).Status; got != domain.EvidenceAdded {
		t.Fatalf("added = %s", got)
	}
	if got := electricalDiff(int64Ptr(1), nil, &domain.MeasurementWindow{}, nil).Status; got != domain.EvidenceRemoved {
		t.Fatalf("removed = %s", got)
	}
}

func TestDiffElectricalUnavailableWhenReferencedMeasurementFailsToResolve(t *testing.T) {
	// Both sides captured a MeasurementID, but the referenced window could
	// not be resolved (e.g. store lookup failed) -- must not be treated as
	// "not captured".
	got := electricalDiff(int64Ptr(1), int64Ptr(2), nil, nil).Status
	if got != domain.EvidenceUnavailable {
		t.Fatalf("status = %s, want UNAVAILABLE", got)
	}
}

func TestDiffSoftwareNotCapturedWhenBothNil(t *testing.T) {
	from := domain.PhysicalCommit{}
	to := domain.PhysicalCommit{}
	if got := softwareDiff(from, to).Status; got != domain.EvidenceNotCaptured {
		t.Fatalf("status = %s", got)
	}
}

func TestDiffSoftwareChangedWhenPopulated(t *testing.T) {
	from := domain.PhysicalCommit{SoftwareRevision: stringPtr("abc123")}
	to := domain.PhysicalCommit{SoftwareRevision: stringPtr("def456")}
	diff := softwareDiff(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Fields) != 1 || diff.Fields[0].Field != "software_revision" {
		t.Fatalf("diff = %#v", diff)
	}
}

func visionOf(components ...domain.VisionComponent) *domain.PhysicalCommitVisionAnalysis {
	return &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: components}}
}

// TestDiffVisionAnalysesNeitherExists: no Gemini call, no stored analysis on
// either side -- NOT_CAPTURED, not UNCHANGED (there is no interpretation to
// compare, not an observed absence of change).
func TestDiffVisionAnalysesNeitherExists(t *testing.T) {
	if got := DiffVisionAnalyses(nil, nil).Status; got != domain.EvidenceNotCaptured {
		t.Fatalf("status = %s", got)
	}
}

func TestDiffVisionAnalysesOnlyFromExists(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "HC-SR04"})
	if got := DiffVisionAnalyses(from, nil).Status; got != domain.EvidenceUnavailable {
		t.Fatalf("status = %s", got)
	}
}

func TestDiffVisionAnalysesOnlyToExists(t *testing.T) {
	to := visionOf(domain.VisionComponent{Name: "HC-SR04"})
	if got := DiffVisionAnalyses(nil, to).Status; got != domain.EvidenceUnavailable {
		t.Fatalf("status = %s", got)
	}
}

func TestDiffVisionAnalysesIdenticalComponentSetsAreUnchanged(t *testing.T) {
	from := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04"}, domain.VisionComponent{Name: "ESP32"})
	to := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04"}, domain.VisionComponent{Name: "ESP32"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

func TestDiffVisionAnalysesComponentAdded(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "ESP32"})
	to := visionOf(domain.VisionComponent{Name: "ESP32"}, domain.VisionComponent{Name: "SG90 Servo"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	change := diff.Changes[0]
	if change.Status != domain.EvidenceAdded || change.Name != "SG90 Servo" || change.BeforeCount != 0 || change.AfterCount != 1 {
		t.Fatalf("change = %#v", change)
	}
}

func TestDiffVisionAnalysesComponentRemoved(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "ESP32"}, domain.VisionComponent{Name: "SG90 Servo"})
	to := visionOf(domain.VisionComponent{Name: "ESP32"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	change := diff.Changes[0]
	if change.Status != domain.EvidenceRemoved || change.Name != "SG90 Servo" || change.BeforeCount != 1 || change.AfterCount != 0 {
		t.Fatalf("change = %#v", change)
	}
}

func TestDiffVisionAnalysesCountIncreased(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "SG90 Servo"})
	to := visionOf(domain.VisionComponent{Name: "SG90 Servo"}, domain.VisionComponent{Name: "SG90 Servo"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	change := diff.Changes[0]
	if change.Status != domain.EvidenceChanged || change.BeforeCount != 1 || change.AfterCount != 2 {
		t.Fatalf("change = %#v", change)
	}
}

func TestDiffVisionAnalysesCountDecreased(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "SG90 Servo"}, domain.VisionComponent{Name: "SG90 Servo"})
	to := visionOf(domain.VisionComponent{Name: "SG90 Servo"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	change := diff.Changes[0]
	if change.Status != domain.EvidenceChanged || change.BeforeCount != 2 || change.AfterCount != 1 {
		t.Fatalf("change = %#v", change)
	}
}

// TestDiffVisionAnalysesMatchesByCatalogIDDespiteDifferentDisplayName: the
// same CatalogID on both sides is identity, even when Gemini's free-text
// Name differs.
func TestDiffVisionAnalysesMatchesByCatalogIDDespiteDifferentDisplayName(t *testing.T) {
	from := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04"})
	to := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04 Ultrasonic Sensor"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

// TestDiffVisionAnalysesMatchesByNormalizedNameWhenCatalogIDAbsent: names
// that differ only by case/whitespace normalize to the same identity when
// neither side has a CatalogID.
func TestDiffVisionAnalysesMatchesByNormalizedNameWhenCatalogIDAbsent(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "SG90 Servo"})
	to := visionOf(domain.VisionComponent{Name: "sg90 servo"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

// TestDiffVisionAnalysesUnrelatedNamesAreNotGuessedSame: "Servo" must never
// be assumed identical to "SG90 Servo" just because they sound related --
// this must report a REMOVE + ADD, never a silent match.
func TestDiffVisionAnalysesUnrelatedNamesAreNotGuessedSame(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "Servo"})
	to := visionOf(domain.VisionComponent{Name: "SG90 Servo"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 2 {
		t.Fatalf("diff = %#v", diff)
	}
	var sawRemoved, sawAdded bool
	for _, change := range diff.Changes {
		switch change.Status {
		case domain.EvidenceRemoved:
			sawRemoved = true
		case domain.EvidenceAdded:
			sawAdded = true
		}
	}
	if !sawRemoved || !sawAdded {
		t.Fatalf("expected one REMOVED and one ADDED, got %#v", diff.Changes)
	}
}

// TestDiffVisionAnalysesConfidenceOnlyChangeIsUnchanged: confidence
// improving/degrading must never, by itself, report a physical change.
func TestDiffVisionAnalysesConfidenceOnlyChangeIsUnchanged(t *testing.T) {
	from := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.91})
	to := visionOf(domain.VisionComponent{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.97})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

func TestDiffVisionAnalysesOrderingDoesNotMatter(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "ESP32"}, domain.VisionComponent{Name: "HC-SR04"})
	to := visionOf(domain.VisionComponent{Name: "HC-SR04"}, domain.VisionComponent{Name: "ESP32"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

func TestDiffVisionAnalysesDuplicateIdenticalComponentsCountCorrectly(t *testing.T) {
	from := visionOf(domain.VisionComponent{Name: "LED"}, domain.VisionComponent{Name: "LED"}, domain.VisionComponent{Name: "LED"})
	to := visionOf(domain.VisionComponent{Name: "LED"}, domain.VisionComponent{Name: "LED"})
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceChanged || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	if diff.Changes[0].BeforeCount != 3 || diff.Changes[0].AfterCount != 2 {
		t.Fatalf("change = %#v", diff.Changes[0])
	}
}

// TestDiffVisionAnalysesRelationshipsNeverCompared: relationships/wiring are
// explicitly out of scope for this milestone -- differing Relationships
// must never affect the component-only semantic visual diff.
func TestDiffVisionAnalysesRelationshipsNeverCompared(t *testing.T) {
	from := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{
		Status:        "VISION_COMPLETE",
		Components:    []domain.VisionComponent{{Name: "SG90 Servo"}},
		Relationships: []domain.VisionRelationship{{From: "SG90 Servo", To: "GPIO13", Role: "signal"}},
	}}
	to := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{
		Status:     "VISION_COMPLETE",
		Components: []domain.VisionComponent{{Name: "SG90 Servo"}},
	}}
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}

// TestDiffVisionAnalysesWarningsNeverCompared: differing Warnings must
// never affect the component-only semantic visual diff.
func TestDiffVisionAnalysesWarningsNeverCompared(t *testing.T) {
	from := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{
		Status:     "VISION_COMPLETE",
		Components: []domain.VisionComponent{{Name: "SG90 Servo"}},
		Warnings:   []string{"Vision findings are AI suggestions and require user confirmation."},
	}}
	to := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{
		Status:     "VISION_COMPLETE",
		Components: []domain.VisionComponent{{Name: "SG90 Servo"}},
	}}
	diff := DiffVisionAnalyses(from, to)
	if diff.Status != domain.EvidenceUnchanged || len(diff.Changes) != 0 {
		t.Fatalf("diff = %#v", diff)
	}
}
