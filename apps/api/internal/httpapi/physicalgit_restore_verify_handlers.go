package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalgit"
)

// resolveReferenceCommit resolves an optional "reference commit" query
// parameter (used by both Restore and Verify as the current/observation
// commit to compare target against). When the parameter is absent, it
// defaults to the project's most recently created Physical Commit other
// than target -- never target itself, and never a fabricated commit.
// Returns (nil, nil) when no reference commit is available/selected, which
// is a valid, expected state, not an error.
func (controller *Controller) resolveReferenceCommit(ctx *fiber.Ctx, project *domain.Project, commitRepository domain.PhysicalCommitRepository, target *domain.PhysicalCommit, queryParam string) (*domain.PhysicalCommit, error) {
	if id := ctx.Query(queryParam); id != "" {
		if !physicalCommitIDPattern.MatchString(id) {
			return nil, apiError(ctx, fiber.StatusBadRequest, "INVALID_PHYSICAL_COMMIT_ID", "The reference physical commit id is malformed.")
		}
		resolved, err := commitRepository.GetPhysicalCommit(project.ID, id)
		if err != nil {
			return nil, internalError(ctx, err)
		}
		if resolved == nil {
			return nil, apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
		}
		return resolved, nil
	}
	items, err := commitRepository.ListPhysicalCommits(project.ID)
	if err != nil {
		return nil, internalError(ctx, err)
	}
	for index := range items {
		if items[index].ID != target.ID {
			return &items[index], nil
		}
	}
	return nil, nil
}

// resolveMeasurement is a small helper shared by restore/verify: it resolves
// a commit's MeasurementID into the full MeasurementWindow, never fabricated
// and never failing the request when resolution is unavailable.
func (controller *Controller) resolveMeasurement(ctx *fiber.Ctx, measurementID *int64) *domain.MeasurementWindow {
	if measurementID == nil {
		return nil
	}
	passportRepository, _, err := controller.passportStorage(ctx)
	if err != nil {
		return nil
	}
	measurement, err := passportRepository.GetMeasurement(*measurementID)
	if err != nil {
		return nil
	}
	return measurement
}

func (controller *Controller) resolveVisionAnalysis(project *domain.Project, commit *domain.PhysicalCommit) *domain.PhysicalCommitVisionAnalysis {
	if commit == nil {
		return nil
	}
	visionRepository, ok := controller.repository.(domain.PhysicalCommitVisionAnalysisRepository)
	if !ok {
		return nil
	}
	record, err := visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, commit.ID)
	if err != nil {
		return nil
	}
	return record
}

// restorePhysicalCommit builds a deterministic restoration checklist for
// returning the project's observable state to target's captured state. It
// never physically modifies hardware and never invokes Gemini -- see
// physicalgit.Restore's doc comment.
func (controller *Controller) restorePhysicalCommit(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	commitID := ctx.Params("commitId")
	if !physicalCommitIDPattern.MatchString(commitID) {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_PHYSICAL_COMMIT_ID", "The physical commit id is malformed.")
	}
	commitRepository, err := controller.physicalCommitRepository(ctx)
	if err != nil {
		return err
	}
	target, err := commitRepository.GetPhysicalCommit(project.ID, commitID)
	if err != nil {
		return internalError(ctx, err)
	}
	if target == nil {
		return apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
	}

	source, restErr := controller.resolveReferenceCommit(ctx, project, commitRepository, target, "source")
	if restErr != nil {
		return restErr
	}

	targetMeasurement := controller.resolveMeasurement(ctx, target.MeasurementID)
	var sourceMeasurement *domain.MeasurementWindow
	if source != nil {
		sourceMeasurement = controller.resolveMeasurement(ctx, source.MeasurementID)
	}
	targetVision := controller.resolveVisionAnalysis(project, target)
	sourceVision := controller.resolveVisionAnalysis(project, source)

	return ctx.JSON(physicalgit.Restore(*target, source, targetMeasurement, sourceMeasurement, targetVision, sourceVision))
}

// verifyPhysicalCommitRestoration deterministically asks whether newly
// observed evidence supports having restored the project to target's
// captured state. It never mutates target and never invokes Gemini -- see
// physicalgit.Verify's doc comment.
func (controller *Controller) verifyPhysicalCommitRestoration(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	commitID := ctx.Params("commitId")
	if !physicalCommitIDPattern.MatchString(commitID) {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_PHYSICAL_COMMIT_ID", "The physical commit id is malformed.")
	}
	commitRepository, err := controller.physicalCommitRepository(ctx)
	if err != nil {
		return err
	}
	target, err := commitRepository.GetPhysicalCommit(project.ID, commitID)
	if err != nil {
		return internalError(ctx, err)
	}
	if target == nil {
		return apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
	}

	observation, restErr := controller.resolveReferenceCommit(ctx, project, commitRepository, target, "observed")
	if restErr != nil {
		return restErr
	}

	targetMeasurement := controller.resolveMeasurement(ctx, target.MeasurementID)
	var currentMeasurement *domain.MeasurementWindow
	if _, measurementsRepository, err := controller.passportStorage(ctx); err == nil {
		if latest, err := measurementsRepository.ListMeasurements(project.ID, 1); err == nil && len(latest) > 0 {
			currentMeasurement = &latest[0]
		}
	}

	// CurrentProfile is the project's LIVE current Circuit Map, not a
	// commit snapshot -- Verify asks whether the live state matches target.
	currentProfile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}

	var observationImage *domain.ProjectMedia
	if observation != nil {
		observationImage = observation.Image
	}

	input := physicalgit.VerifyInput{
		CurrentProfile:     currentProfile,
		TargetMeasurement:  targetMeasurement,
		CurrentMeasurement: currentMeasurement,
		ObservationImage:   observationImage,
		ObservationVision:  controller.resolveVisionAnalysis(project, observation),
		TargetVision:       controller.resolveVisionAnalysis(project, target),
	}
	return ctx.JSON(physicalgit.Verify(*target, input))
}
