// Package diagnosticcontext builds a small, bounded, provenance-tagged
// slice of domain.EvidenceFact from a project's Physical Git history,
// AI-derived vision interpretation, software intent, and matched Component
// Catalog specs -- ready to merge into diagnostics.Engine's Evidence via
// AnalyzeEnvelopeWithContext.
//
// This package performs no reasoning: it only selects and labels already
// -deterministic facts (the physical-history lines it uses come from
// internal/physicalgit's own diagnostic_context, which is itself
// deterministic and non-causal). It never asserts that one observed change
// caused another, and it never lets AI-derived interpretation (vision) or
// software-declared intent masquerade as measured electrical evidence --
// each fact keeps the domain.Provenance that says exactly where it came
// from.
package diagnosticcontext

import (
	"fmt"
	"sort"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalgit"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

// maxFacts bounds the total number of facts this package ever returns,
// regardless of how much Physical Git history, software analysis, or
// catalog data a project accumulates. This is what keeps a Gemini prompt
// built from this context from growing without bound as history grows.
const maxFacts = 16

const (
	maxVisionComponents = 5
	maxSoftwarePins     = 6
	maxCatalogSpecs     = 5
)

// Build never returns an error: every source it reads is optional and a
// missing/unavailable one simply contributes no facts, exactly like every
// other Physical Git evidence source in this codebase.
func Build(repository domain.Repository, project domain.Project, catalog []componentcatalog.Entry) []domain.EvidenceFact {
	var facts []domain.EvidenceFact

	profile, _ := repository.GetProfile(project.ID)

	facts = append(facts, physicalHistoryFacts(repository, project)...)
	facts = append(facts, softwareIntentFacts(project)...)
	facts = append(facts, catalogFacts(profile, catalog)...)

	if len(facts) > maxFacts {
		facts = facts[:maxFacts]
	}
	return facts
}

// physicalHistoryFacts compares the two most recent Physical Commits (when
// both exist) using the exact same deterministic physicalgit.Diff already
// used by the Physical Diff feature -- it does not reimplement that
// comparison. Only the diff's own bounded, non-causal DiagnosticContext
// lines are used; nothing else about the diff is included here (the diff
// itself remains available through the existing Physical Diff endpoint).
// Vision interpretation of the latest commit's image, if any, is included
// separately and labeled AI_INTERPRETATION -- never MEASURED.
func physicalHistoryFacts(repository domain.Repository, project domain.Project) []domain.EvidenceFact {
	commitRepository, ok := repository.(domain.PhysicalCommitRepository)
	if !ok {
		return nil
	}
	commits, err := commitRepository.ListPhysicalCommits(project.ID)
	if err != nil || len(commits) == 0 {
		return nil
	}
	// ListPhysicalCommits returns newest first (see its own contract).
	latest := commits[0]

	var facts []domain.EvidenceFact
	if len(commits) >= 2 {
		previous := commits[1]
		var fromMeasurement, toMeasurement *domain.MeasurementWindow
		var fromVision, toVision *domain.PhysicalCommitVisionAnalysis
		if passportRepository, ok := repository.(domain.PassportRepository); ok {
			if previous.MeasurementID != nil {
				fromMeasurement, _ = passportRepository.GetMeasurement(*previous.MeasurementID)
			}
			if latest.MeasurementID != nil {
				toMeasurement, _ = passportRepository.GetMeasurement(*latest.MeasurementID)
			}
		}
		if visionRepository, ok := repository.(domain.PhysicalCommitVisionAnalysisRepository); ok {
			fromVision, _ = visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, previous.ID)
			toVision, _ = visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, latest.ID)
		}
		diff := physicalgit.Diff(previous, latest, fromMeasurement, toMeasurement, fromVision, toVision)
		for _, line := range diff.DiagnosticContext {
			facts = append(facts, domain.EvidenceFact{
				Name:       "physical_history_change",
				Value:      line,
				Provenance: domain.ProvenancePhysicalHistory,
			})
		}
	}

	facts = append(facts, visionFacts(repository, project, latest)...)
	return facts
}

func visionFacts(repository domain.Repository, project domain.Project, commit domain.PhysicalCommit) []domain.EvidenceFact {
	visionRepository, ok := repository.(domain.PhysicalCommitVisionAnalysisRepository)
	if !ok {
		return nil
	}
	analysis, err := visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, commit.ID)
	if err != nil || analysis == nil || analysis.Analysis.Status != "VISION_COMPLETE" {
		return nil
	}
	components := append([]domain.VisionComponent(nil), analysis.Analysis.Components...)
	sort.SliceStable(components, func(i, j int) bool { return components[i].Confidence > components[j].Confidence })
	if len(components) > maxVisionComponents {
		components = components[:maxVisionComponents]
	}
	facts := make([]domain.EvidenceFact, 0, len(components))
	for _, component := range components {
		facts = append(facts, domain.EvidenceFact{
			Name:       "vision_interpreted_component",
			Value:      component.Name,
			Detail:     fmt.Sprintf("AI-interpreted from %s's saved image, %.0f%% confidence -- not measured evidence.", commit.DisplayID, component.Confidence*100),
			Provenance: domain.ProvenanceAIInterpretation,
		})
	}
	return facts
}

// softwareIntentFacts reports what the project's analyzed firmware source
// declares about pin usage -- intent, not proof of physical wiring. Absent
// when no repository/code has been analyzed (GitHub is optional).
func softwareIntentFacts(project domain.Project) []domain.EvidenceFact {
	if project.Analysis == nil {
		return nil
	}
	pins := project.Analysis.Code.Pins
	if len(pins) > maxSoftwarePins {
		pins = pins[:maxSoftwarePins]
	}
	facts := make([]domain.EvidenceFact, 0, len(pins))
	for _, pin := range pins {
		facts = append(facts, domain.EvidenceFact{
			Name:       "software_pin_mapping",
			Value:      fmt.Sprintf("GPIO%d", pin.GPIO),
			Detail:     fmt.Sprintf("firmware declares %s as %s (%s)", pin.Symbol, pin.Direction, pin.Behavior),
			Provenance: domain.ProvenanceSoftware,
		})
	}
	return facts
}

// catalogFacts includes only specs for components the live ProjectProfile
// actually has, matched by the existing Component Catalog lookup -- never
// the whole catalog, and never a fact the catalog does not actually state.
func catalogFacts(profile *domain.ProjectProfile, catalog []componentcatalog.Entry) []domain.EvidenceFact {
	if profile == nil || len(catalog) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var facts []domain.EvidenceFact
	for _, component := range profile.Components {
		entry, ok := componentcatalog.Find(catalog, component.Name)
		if !ok || seen[entry.ID] {
			continue
		}
		seen[entry.ID] = true
		facts = append(facts, domain.EvidenceFact{
			Name:       "catalog_spec",
			Value:      entry.Name,
			Detail:     entry.ExpectedBehavior,
			Provenance: domain.ProvenanceSpecification,
		})
		if len(facts) >= maxCatalogSpecs {
			break
		}
	}
	return facts
}
