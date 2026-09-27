package diagnosticcontext

import (
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/store"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

func openTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func gpio(value int) *int { return &value }

func baseProfile(broken bool) domain.ProjectProfile {
	echoGPIO := 18
	components := []domain.ComponentSpecification{{ID: "hc-sr04-1", Name: "HC-SR04"}}
	if broken {
		echoGPIO = 19
		components = append(components, domain.ComponentSpecification{ID: "sg90-1", Name: "SG90 Servo"})
	}
	return domain.ProjectProfile{
		ID: "proj-1", ProjectName: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3, Confirmed: true,
		Components: components,
		Connections: []domain.ProfileConnection{
			{ID: "echo", ComponentName: "HC-SR04", Role: "ECHO", GPIO: gpio(echoGPIO), Target: "ESP32", Direction: "input", Behavior: "digital_pulse", Sources: []domain.ProjectFactSource{domain.SourceUser}},
		},
	}
}

func TestBuildReturnsNoFactsForAProjectWithNoHistory(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	facts := Build(repository, project, nil)
	if len(facts) != 0 {
		t.Fatalf("facts = %#v, want none for a project with no commits/software/catalog", facts)
	}
}

func TestBuildIncludesBoundedNonCausalPhysicalHistory(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(baseProfile(false)); err != nil {
		t.Fatal(err)
	}

	first, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000001", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(false))})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000002", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(true))})
	if err != nil {
		t.Fatal(err)
	}
	_ = first
	_ = second

	facts := Build(repository, project, nil)
	if len(facts) == 0 {
		t.Fatal("expected physical-history facts from the diff between the two commits")
	}
	for _, fact := range facts {
		if fact.Provenance != domain.ProvenancePhysicalHistory {
			t.Fatalf("fact = %#v, want PHYSICAL_HISTORY provenance", fact)
		}
		text, ok := fact.Value.(string)
		if !ok {
			t.Fatalf("fact.Value = %#v, want a string", fact.Value)
		}
		for _, forbidden := range []string{"caused", "because", "due to", "led to"} {
			if containsFold(text, forbidden) {
				t.Fatalf("fact %q asserts causation via %q -- physical history must stay factual, never causal", text, forbidden)
			}
		}
	}
	if len(facts) > maxFacts {
		t.Fatalf("facts = %d entries, want at most %d (bounded)", len(facts), maxFacts)
	}
}

func TestBuildLabelsVisionInterpretationDistinctlyFromMeasuredEvidence(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(baseProfile(false)); err != nil {
		t.Fatal(err)
	}
	first, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000001", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(false))})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000002", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(true))})
	if err != nil {
		t.Fatal(err)
	}
	_ = first
	if _, err := repository.SavePhysicalCommitVisionAnalysis(domain.PhysicalCommitVisionAnalysis{
		ID: "pcvision-1", ProjectID: project.ID, PhysicalCommitID: second.ID, Provider: "gemini",
		Analysis: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "SG90 Servo", Confidence: 0.87, Source: domain.SourceVisionAI}}},
	}); err != nil {
		t.Fatal(err)
	}

	facts := Build(repository, project, nil)
	found := false
	for _, fact := range facts {
		if fact.Name != "vision_interpreted_component" {
			continue
		}
		found = true
		if fact.Provenance != domain.ProvenanceAIInterpretation {
			t.Fatalf("vision fact provenance = %s, want AI_INTERPRETATION so it can never be mistaken for measured evidence", fact.Provenance)
		}
	}
	if !found {
		t.Fatal("expected a vision_interpreted_component fact for the commit with a persisted vision analysis")
	}
}

func TestBuildIncludesSoftwareIntentLabeledAsSoftware(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{
		ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3,
		Analysis: &domain.ProjectAnalysis{Code: domain.CodeAnalysis{Pins: []domain.CodePinFinding{{GPIO: 18, Symbol: "ECHO_PIN", Direction: "input", Behavior: "digital_pulse"}}}},
	}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	facts := Build(repository, project, nil)
	if len(facts) != 1 || facts[0].Provenance != domain.ProvenanceSoftware || facts[0].Value != "GPIO18" {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestBuildIncludesOnlyCatalogSpecsForComponentsActuallyInTheProfile(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(baseProfile(false)); err != nil {
		t.Fatal(err)
	}
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	facts := Build(repository, project, catalog)
	if len(facts) != 1 || facts[0].Provenance != domain.ProvenanceSpecification {
		t.Fatalf("facts = %#v, want exactly one catalog_spec fact for HC-SR04", facts)
	}
}

func TestBuildNeverExceedsMaxFactsEvenWithManyContributingSources(t *testing.T) {
	repository := openTestStore(t)
	project := domain.Project{
		ID: "proj-1", Name: "Distance Alarm", Controller: "ESP32", LogicVoltage: 3.3,
		Analysis: &domain.ProjectAnalysis{Code: domain.CodeAnalysis{Pins: []domain.CodePinFinding{
			{GPIO: 1, Symbol: "A"}, {GPIO: 2, Symbol: "B"}, {GPIO: 3, Symbol: "C"}, {GPIO: 4, Symbol: "D"},
			{GPIO: 5, Symbol: "E"}, {GPIO: 6, Symbol: "F"}, {GPIO: 7, Symbol: "G"}, {GPIO: 8, Symbol: "H"},
		}}},
	}
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(baseProfile(false)); err != nil {
		t.Fatal(err)
	}
	first, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000001", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(false))})
	if err != nil {
		t.Fatal(err)
	}
	_ = first
	second, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-00000000000000000000000000000002", ProjectID: project.ID, ProfileSnapshot: profilePtr(baseProfile(true))})
	if err != nil {
		t.Fatal(err)
	}
	_ = second
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	facts := Build(repository, project, catalog)
	if len(facts) > maxFacts {
		t.Fatalf("facts = %d, want at most %d regardless of how many sources contribute", len(facts), maxFacts)
	}
}

func profilePtr(profile domain.ProjectProfile) *domain.ProjectProfile { return &profile }

func containsFold(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			a, b := haystack[i+j], needle[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
