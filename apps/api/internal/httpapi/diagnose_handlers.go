package httpapi

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnosticcontext"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
)

// createProjectMeasurement lets a real project (one created via POST
// /api/v1/projects, not the single global demo/simulator profile) record a
// telemetry measurement over HTTP -- the missing link that let Physical
// Git/camera/vision evidence accumulate for a project that never has a live
// TelemetrySource. It reuses the exact same deterministic signalanalysis
// step the built-in loop uses (Engine.AnalyzeSignals) and the exact same
// idempotent, capacity-checked SaveMeasurement -- no parallel measurement
// pipeline is introduced.
func (controller *Controller) createProjectMeasurement(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	measurements, ok := controller.repository.(domain.MeasurementRepository)
	if !ok {
		return apiError(ctx, fiber.StatusNotImplemented, "MEASUREMENT_STORAGE_UNAVAILABLE", "Measurement storage is unavailable.")
	}
	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_NOT_FOUND", "This project has no Project Profile yet; analyze it before submitting measurements.")
	}

	var input struct {
		Source   string                   `json:"source"`
		Envelope domain.TelemetryEnvelope `json:"envelope"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "The measurement must be valid JSON.")
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "serial"
	}
	if _, ok := passport.SourceKind(source); !ok {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_MEASUREMENT_SOURCE", "source must be \"serial\" or \"simulator\".")
	}
	// A caller can only ever submit a measurement under its own project's
	// profile id -- an envelope that claims a different project's profile
	// id is rejected outright rather than silently corrected, so a
	// misconfigured device can never write into another project's history.
	if input.Envelope.ProfileID == "" {
		input.Envelope.ProfileID = project.ID
	} else if input.Envelope.ProfileID != project.ID {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "PROFILE_ID_MISMATCH", "envelope.profile_id must match this project's id.")
	}
	if len(input.Envelope.Samples) == 0 {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "EMPTY_ENVELOPE", "The measurement has no samples.")
	}

	analysis, err := controller.engine.AnalyzeSignals(input.Envelope, *profile)
	if err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_TELEMETRY", err.Error())
	}

	window := domain.MeasurementWindow{
		ProfileID:    project.ID,
		Source:       source,
		DeviceID:     input.Envelope.DeviceID,
		Sequence:     input.Envelope.Sequence,
		CapturedAtMS: input.Envelope.CapturedAtMS,
		Raw:          input.Envelope,
		Analysis:     analysis,
	}
	stored, err := measurements.SaveMeasurement(window)
	if err != nil {
		return apiError(ctx, fiber.StatusConflict, "MEASUREMENT_CONFLICT", err.Error())
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

// getProjectDiagnosis is the per-project counterpart to the built-in demo's
// GET /api/v1/session. It is never persisted separately -- every call
// recomputes it from whatever is currently persisted for the project (its
// ProjectProfile, its most recent MeasurementWindows, and its Physical Git
// history), the same "derive, don't cache" pattern Restore and Verify
// already use. When no measurement has been captured yet, it honestly
// reports INSUFFICIENT_EVIDENCE with whatever non-electrical context is
// available instead of fabricating a diagnosis.
func (controller *Controller) getProjectDiagnosis(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}

	physicalContext := diagnosticcontext.Build(controller.repository, *project, controller.catalog)

	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}

	measurementRepository, hasMeasurements := controller.repository.(domain.MeasurementRepository)
	var recent []domain.MeasurementWindow
	if hasMeasurements {
		recent, err = measurementRepository.ListMeasurements(project.ID, 2)
		if err != nil {
			return internalError(ctx, err)
		}
	}

	if profile == nil || len(recent) == 0 {
		unresolved := []string{}
		if profile == nil {
			unresolved = append(unresolved, "This project has no confirmed Project Profile yet.")
		}
		if len(recent) == 0 {
			unresolved = append(unresolved, "No measurement has been captured for this project yet.")
		}
		return ctx.JSON(domain.ProjectDiagnosis{
			ProjectID: project.ID,
			Status:    domain.ProjectDiagnosisInsufficientEvidence,
			Evidence: domain.Evidence{
				Expected:            map[string]any{},
				Observed:            map[string]any{},
				Baseline:            map[string]any{"status": domain.BaselineUnknown},
				UnresolvedQuestions: unresolved,
				PhysicalContext:     physicalContext,
			},
			Diagnosis: domain.Diagnosis{
				Headline:       "Insufficient electrical evidence",
				Summary:        "No measurement has been analyzed for this project yet, so no electrical diagnosis can be made.",
				PossibleCauses: []string{},
				Confidence:     0,
				NextTest:       "Capture a measurement for this project (POST .../measurements) and try again.",
			},
			GeneratedAtMS: time.Now().UnixMilli(),
		})
	}

	current := recent[0]
	analyzedProfile := *profile
	if baselineSource, ok := passport.SourceKind(current.Source); ok {
		if passportRepository, ok := controller.repository.(domain.PassportRepository); ok {
			stored, baselineErr := passportRepository.LatestKnownGood(project.ID, baselineSource, current.DeviceID)
			if baselineErr == nil {
				analyzedProfile = passport.ApplyKnownGood(analyzedProfile, baselineSource, stored)
			}
		}
	}

	var reference *domain.AnalysisResult
	var referenceMeasurementID *int64
	if len(recent) >= 2 {
		referenceAnalysis := recent[1].Analysis
		reference = &referenceAnalysis
		referenceMeasurementID = &recent[1].ID
	}

	session, err := controller.engine.DiagnoseAnalysis(ctx.Context(), analyzedProfile, domain.StageDiagnose, current.Source, current.Analysis, reference, physicalContext)
	if err != nil {
		return internalError(ctx, err)
	}

	measurementID := current.ID
	return ctx.JSON(domain.ProjectDiagnosis{
		ProjectID:              project.ID,
		Status:                 domain.ProjectDiagnosisComplete,
		Evidence:               session.Evidence,
		Diagnosis:              session.Diagnosis,
		MeasurementID:          &measurementID,
		ReferenceMeasurementID: referenceMeasurementID,
		GeneratedAtMS:          time.Now().UnixMilli(),
	})
}
