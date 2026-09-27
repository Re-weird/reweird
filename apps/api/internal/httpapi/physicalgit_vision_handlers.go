package httpapi

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/projects"
)

func randomPhysicalCommitVisionAnalysisID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "pcvision-" + hex.EncodeToString(raw[:]), nil
}

func (controller *Controller) physicalCommitVisionRepository(ctx *fiber.Ctx) (domain.PhysicalCommitVisionAnalysisRepository, error) {
	repository, ok := controller.repository.(domain.PhysicalCommitVisionAnalysisRepository)
	if !ok {
		return nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "PHYSICAL_GIT_VISION_STORAGE_UNAVAILABLE"})
	}
	return repository, nil
}

// analyzePhysicalCommitHardware runs ReWeird's existing Gemini Vision
// pipeline against a Physical Commit's already-stored raw image and
// persists the result as a separate interpretation record keyed by commit
// id. It never mutates the commit, its image, or the ProjectProfile
// snapshot. Only a genuinely successful result is persisted -- a skipped
// (Gemini not configured) or failed attempt is neither evidence nor
// interpretation, so it is never stored and never overwrites a prior
// successful analysis.
func (controller *Controller) analyzePhysicalCommitHardware(ctx *fiber.Ctx) error {
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
	commit, err := commitRepository.GetPhysicalCommit(project.ID, commitID)
	if err != nil {
		return internalError(ctx, err)
	}
	if commit == nil {
		return apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
	}
	if commit.Image == nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "NO_IMAGE_CAPTURED", "This commit has no captured image; hardware analysis is unavailable.")
	}
	if controller.understanding == nil {
		return apiError(ctx, fiber.StatusServiceUnavailable, "ANALYSIS_UNAVAILABLE", "Project understanding is not configured.")
	}
	imagePath, err := projects.ResolveImage(controller.uploadRoot, commit.Image.StorageRef)
	if err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_IMAGE_REFERENCE", "The stored image reference is invalid.")
	}

	result := controller.understanding.AnalyzeCommitImage(ctx.Context(), imagePath, commit.Image.ContentType)
	switch result.Status {
	case "VISION_COMPLETE":
		// only a genuine success is persisted -- fall through below.
	case "VISION_SKIPPED":
		// Gemini is not configured. Neither evidence nor interpretation --
		// never persisted, never overwrites a prior successful analysis.
		return apiError(ctx, fiber.StatusServiceUnavailable, "ANALYSIS_UNAVAILABLE", "Gemini Vision is not configured.")
	default:
		// VISION_FAILED (transport error, timeout, invalid response, or the
		// stored image file is missing/corrupt). Never persisted, never
		// overwrites a prior successful analysis.
		return apiError(ctx, fiber.StatusBadGateway, "VISION_ANALYSIS_FAILED", "Gemini Vision did not return a usable result. Any previous analysis for this commit was not changed.")
	}

	id, err := randomPhysicalCommitVisionAnalysisID()
	if err != nil {
		return internalError(ctx, err)
	}
	visionRepository, err := controller.physicalCommitVisionRepository(ctx)
	if err != nil {
		return err
	}
	stored, err := visionRepository.SavePhysicalCommitVisionAnalysis(domain.PhysicalCommitVisionAnalysis{
		ID:               id,
		ProjectID:        project.ID,
		PhysicalCommitID: commit.ID,
		Provider:         "gemini",
		Analysis:         result,
	})
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

// getPhysicalCommitVisionAnalysis only ever reads from the repository -- it
// never references controller.understanding, so it cannot invoke Gemini
// even by mistake.
func (controller *Controller) getPhysicalCommitVisionAnalysis(ctx *fiber.Ctx) error {
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
	commit, err := commitRepository.GetPhysicalCommit(project.ID, commitID)
	if err != nil {
		return internalError(ctx, err)
	}
	if commit == nil {
		return apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
	}
	visionRepository, err := controller.physicalCommitVisionRepository(ctx)
	if err != nil {
		return err
	}
	record, err := visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, commit.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if record == nil {
		return apiError(ctx, fiber.StatusNotFound, "VISION_ANALYSIS_NOT_FOUND", "This commit has not been analyzed yet.")
	}
	return ctx.JSON(record)
}
