package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"mime/multipart"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/cameracapture"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalgit"
	"github.com/re-weird/reweird/apps/api/internal/projects"
)

var physicalCommitIDPattern = regexp.MustCompile(`^pcommit-[a-f0-9]{32}$`)

const maxPhysicalCommitNoteBytes = 2000

func randomPhysicalCommitID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "pcommit-" + hex.EncodeToString(raw[:]), nil
}

func (controller *Controller) physicalCommitRepository(ctx *fiber.Ctx) (domain.PhysicalCommitRepository, error) {
	repository, ok := controller.repository.(domain.PhysicalCommitRepository)
	if !ok {
		return nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "PHYSICAL_GIT_STORAGE_UNAVAILABLE"})
	}
	return repository, nil
}

// createPhysicalCommit captures whatever ReWeird currently knows about a
// project's physical state: the Circuit Map/component state (ProjectProfile),
// the active Device Passport baselines, the most recent measurement, and an
// optional photo and note. Every source is optional and never fabricated --
// a commit is still valid with none of them, which is required for a normal
// project that has no Device Passport or ESP32 telemetry yet.
func (controller *Controller) createPhysicalCommit(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}

	var note string
	var fileHeader *multipart.FileHeader
	if strings.HasPrefix(ctx.Get(fiber.HeaderContentType), fiber.MIMEMultipartForm) {
		note = ctx.FormValue("note")
		if header, ferr := ctx.FormFile("file"); ferr == nil {
			fileHeader = header
		}
	} else {
		var input struct {
			Note string `json:"note"`
		}
		if len(ctx.Body()) > 0 {
			if err := ctx.BodyParser(&input); err != nil {
				return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "The commit note must be valid JSON.")
			}
		}
		note = input.Note
	}
	note = strings.TrimSpace(note)
	if len(note) > maxPhysicalCommitNoteBytes {
		return apiError(ctx, fiber.StatusBadRequest, "NOTE_TOO_LONG", "Keep the commit note under 2000 characters.")
	}

	id, err := randomPhysicalCommitID()
	if err != nil {
		return internalError(ctx, err)
	}

	commit := domain.PhysicalCommit{ID: id, ProjectID: project.ID, Note: note}

	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	commit.ProfileSnapshot = profile

	if passportRepository, measurementsRepository, err := controller.passportStorage(ctx); err == nil {
		// Only the physical baseline is auto-attached. A simulated baseline
		// belongs to the synthetic/game-demo workflow, never to a real
		// Physical Commit's history.
		if physical, err := passportRepository.LatestKnownGoodOfSource(project.ID, domain.BaselinePhysical); err == nil && physical != nil {
			commit.PassportBaselineIDs = append(commit.PassportBaselineIDs, physical.ID)
		}
		if latest, err := measurementsRepository.ListMeasurements(project.ID, 1); err == nil && len(latest) > 0 {
			commit.MeasurementID = &latest[0].ID
		}
	}
	// Device Passport / measurement storage being unavailable never blocks a
	// commit -- both are optional, best-effort evidence, not a dependency.

	if fileHeader != nil {
		file, ferr := fileHeader.Open()
		if ferr != nil {
			return apiError(ctx, fiber.StatusBadRequest, "IMAGE_READ_FAILED", "The uploaded image could not be read.")
		}
		image, saveErr := projects.SavePhysicalCommitImage(controller.uploadRoot, project.ID, commit.ID, file)
		closeErr := file.Close()
		if saveErr != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_IMAGE", saveErr.Error())
		}
		if closeErr != nil {
			return internalError(ctx, closeErr)
		}
		commit.Image = image
	} else if project.CameraConfig != nil {
		// A configured camera is used only when no photo was manually
		// uploaded. Any failure here (not reachable, timeout, invalid
		// stream, or the captured frame failing to save) never blocks the
		// commit -- it simply proceeds with no image, exactly like a
		// project with no camera configured at all. Gemini is never
		// involved in this path.
		if frame, captureErr := cameracapture.Capture(ctx.Context(), project.CameraConfig.URL); captureErr == nil {
			if image, saveErr := projects.SavePhysicalCommitImage(controller.uploadRoot, project.ID, commit.ID, bytes.NewReader(frame.Bytes)); saveErr == nil {
				commit.Image = image
			}
		}
	}

	commitRepository, err := controller.physicalCommitRepository(ctx)
	if err != nil {
		return err
	}
	stored, err := commitRepository.SavePhysicalCommit(commit)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

func (controller *Controller) listPhysicalCommits(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	commitRepository, err := controller.physicalCommitRepository(ctx)
	if err != nil {
		return err
	}
	items, err := commitRepository.ListPhysicalCommits(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"items": items, "count": len(items)})
}

func (controller *Controller) getPhysicalCommit(ctx *fiber.Ctx) error {
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
	return ctx.JSON(commit)
}

// getPhysicalCommitDetail is a separate, additive endpoint -- it does not
// change what GET .../physical-commits/:commitId returns. It resolves the
// commit's referenced MeasurementWindow for display; a resolution failure
// never fails the request and is never fabricated, it is simply omitted.
func (controller *Controller) getPhysicalCommitDetail(ctx *fiber.Ctx) error {
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

	detail := domain.PhysicalCommitDetail{Commit: *commit}
	if commit.MeasurementID != nil {
		if passportRepository, _, err := controller.passportStorage(ctx); err == nil {
			if measurement, err := passportRepository.GetMeasurement(*commit.MeasurementID); err == nil {
				detail.Measurement = measurement
			}
		}
	}
	return ctx.JSON(detail)
}

// diffPhysicalCommits compares two Physical Commits belonging to the same
// project. The comparison itself is entirely deterministic (internal/physicalgit)
// -- no LLM is involved anywhere in this request.
func (controller *Controller) diffPhysicalCommits(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}

	fromID, toID := ctx.Query("from"), ctx.Query("to")
	if !physicalCommitIDPattern.MatchString(fromID) || !physicalCommitIDPattern.MatchString(toID) {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_PHYSICAL_COMMIT_ID", "Both from and to must be valid physical commit ids.")
	}

	commitRepository, err := controller.physicalCommitRepository(ctx)
	if err != nil {
		return err
	}
	from, err := commitRepository.GetPhysicalCommit(project.ID, fromID)
	if err != nil {
		return internalError(ctx, err)
	}
	to, err := commitRepository.GetPhysicalCommit(project.ID, toID)
	if err != nil {
		return internalError(ctx, err)
	}
	if from == nil || to == nil {
		// Identical response regardless of which side is missing -- never
		// leak which id was invalid, matching this codebase's existing
		// not-found-hides-existence convention.
		return apiError(ctx, fiber.StatusNotFound, "PHYSICAL_COMMIT_NOT_FOUND", "No physical commit with this id exists for this project.")
	}

	var fromMeasurement, toMeasurement *domain.MeasurementWindow
	if passportRepository, _, err := controller.passportStorage(ctx); err == nil {
		if from.MeasurementID != nil {
			fromMeasurement, _ = passportRepository.GetMeasurement(*from.MeasurementID)
		}
		if to.MeasurementID != nil {
			toMeasurement, _ = passportRepository.GetMeasurement(*to.MeasurementID)
		}
	}

	// Semantic visual diff compares already-persisted Gemini Vision results
	// only -- this is a plain repository read, never a capability-required
	// helper, and never invokes controller.understanding/Gemini. A backend
	// without vision storage wired simply leaves both sides nil, which
	// DiffVisionAnalyses reports as NOT_CAPTURED.
	var fromVision, toVision *domain.PhysicalCommitVisionAnalysis
	if visionRepository, ok := controller.repository.(domain.PhysicalCommitVisionAnalysisRepository); ok {
		fromVision, _ = visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, from.ID)
		toVision, _ = visionRepository.GetPhysicalCommitVisionAnalysis(project.ID, to.ID)
	}

	return ctx.JSON(physicalgit.Diff(*from, *to, fromMeasurement, toMeasurement, fromVision, toVision))
}
