package httpapi

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/history"
	"github.com/re-weird/reweird/apps/api/internal/passport"
)

type knownGoodRequest struct {
	MeasurementID  int64  `json:"measurement_id"`
	ConfirmHealthy bool   `json:"confirm_healthy"`
	Note           string `json:"note"`
}

func (controller *Controller) passportStorage(ctx *fiber.Ctx) (domain.PassportRepository, domain.MeasurementRepository, error) {
	repository, ok := controller.repository.(domain.PassportRepository)
	if !ok {
		return nil, nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "PASSPORT_STORAGE_UNAVAILABLE"})
	}
	measurements, ok := controller.repository.(domain.MeasurementRepository)
	if !ok {
		return nil, nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "MEASUREMENT_STORAGE_UNAVAILABLE"})
	}
	return repository, measurements, nil
}

func (controller *Controller) saveKnownGood(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	repository, _, err := controller.passportStorage(ctx)
	if err != nil {
		return err
	}
	var request knownGoodRequest
	if err := ctx.BodyParser(&request); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Provide a measurement ID and explicit healthy confirmation.")
	}
	if request.MeasurementID <= 0 || !request.ConfirmHealthy || len(strings.TrimSpace(request.Note)) > 500 {
		return apiError(ctx, fiber.StatusBadRequest, "CONFIRMATION_REQUIRED", "Select a stored capture, explicitly mark it healthy, and keep the note under 500 characters.")
	}
	profile, err := controller.repository.GetProfile(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "No profile exists for this device.")
	}
	if !profile.Confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_UNCONFIRMED", "Confirm the Project Profile before saving a baseline.")
	}
	project, err := controller.repository.GetProject(profile.ProjectID)
	if err != nil {
		return internalError(ctx, err)
	}
	demo := profile.ID == "ultrasonic-demo" && project == nil
	window, err := repository.GetMeasurement(request.MeasurementID)
	if err != nil {
		return internalError(ctx, err)
	}
	if window == nil || window.ProfileID != profile.ID {
		return apiError(ctx, fiber.StatusNotFound, "MEASUREMENT_NOT_FOUND", "That capture does not belong to this profile.")
	}
	if demo && window.Source != "simulator" {
		return apiError(ctx, fiber.StatusConflict, "DEMO_SOURCE_REQUIRED", "The built-in demo accepts simulated baselines only.")
	}
	var record domain.KnownGoodBaseline
	if window.Source == "serial" {
		if project == nil || project.ProbePlan == nil || !project.ProbePlan.Connected || project.ProbePlan.ProfileID != profile.ID {
			return apiError(ctx, fiber.StatusConflict, "PROBES_NOT_CONFIRMED", "Confirm the generated probe plan before saving a physical baseline.")
		}
		if project.ProbePlan.ConnectedAtMS <= 0 || window.IngestedAtMS < project.ProbePlan.ConnectedAtMS {
			return apiError(ctx, fiber.StatusConflict, "CAPTURE_PRECEDES_PROBE_CONFIRMATION", "Take a new physical capture after confirming the probe connections.")
		}
		// A physical Known Good is learned from a run of consecutive REAL
		// SERIAL windows, never from a single capture.
		learned, refusal, err := controller.learnPhysicalKnownGood(*profile, *window, project.ProbePlan.ConnectedAtMS, request.Note)
		if err != nil {
			return internalError(ctx, err)
		}
		if refusal != nil {
			return apiError(ctx, refusal.status, refusal.code, refusal.detail)
		}
		record = learned
	} else {
		record, err = passport.BuildKnownGood(*profile, *window, request.Note)
		if err != nil {
			return apiError(ctx, fiber.StatusUnprocessableEntity, "CAPTURE_NOT_HEALTHY", err.Error())
		}
	}
	stored, err := repository.SaveKnownGood(record)
	if errors.Is(err, domain.ErrKnownGoodExists) {
		return apiError(ctx, fiber.StatusConflict, "BASELINE_ALREADY_SAVED", "This capture has already been saved as Known Good.")
	}
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

func (controller *Controller) devicePassport(ctx *fiber.Ctx) error {
	repository, measurements, err := controller.passportStorage(ctx)
	if err != nil {
		return err
	}
	profile, err := controller.repository.GetProfile(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "No profile exists for this device.")
	}
	if !profile.Confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_UNCONFIRMED", "Confirm the Project Profile before opening its passport.")
	}
	project, err := controller.repository.GetProject(profile.ProjectID)
	if err != nil {
		return internalError(ctx, err)
	}
	allBaselines, err := repository.ListKnownGood(profile.ID, 100)
	if err != nil {
		return internalError(ctx, err)
	}
	windowList, err := measurements.ListMeasurements(profile.ID, 50)
	if err != nil {
		return internalError(ctx, err)
	}
	result := domain.DevicePassport{
		Profile: *profile, Project: project, KnownGood: allBaselines,
		RecentCaptures: make([]domain.PassportCapture, 0), History: make([]domain.HistorySummary, 0),
		VerifiedRepairs: make([]domain.PassportRepair, 0),
	}
	if project != nil {
		result.ProbePlan = project.ProbePlan
	}
	result.PhysicalBaseline, err = repository.LatestKnownGoodOfSource(profile.ID, domain.BaselinePhysical)
	if err != nil {
		return internalError(ctx, err)
	}
	result.SimulatedBaseline, err = repository.LatestKnownGoodOfSource(profile.ID, domain.BaselineSimulated)
	if err != nil {
		return internalError(ctx, err)
	}
	for _, window := range windowList {
		if capture, ok := passport.CaptureSummary(window); ok {
			result.RecentCaptures = append(result.RecentCaptures, capture)
			if len(result.RecentCaptures) == 10 {
				break
			}
		}
	}
	hasPhysical, err := repository.HasPhysicalBaseline(profile.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if len(windowList) > 0 {
		latest := windowList[0]
		if source, ok := passport.SourceKind(latest.Source); ok {
			baseline, err := repository.LatestKnownGood(profile.ID, source, latest.DeviceID)
			if err != nil {
				return internalError(ctx, err)
			}
			result.Status, result.StatusDetail = passport.CurrentStatus(*profile, &latest, baseline, hasPhysical)
		} else {
			result.Status, result.StatusDetail = passport.CurrentStatus(*profile, &latest, nil, hasPhysical)
		}
	} else {
		result.Status, result.StatusDetail = passport.CurrentStatus(*profile, nil, nil, hasPhysical)
	}
	if workflows, ok := controller.repository.(domain.HistoryRepository); ok {
		for offset := 0; offset < 2000 && (len(result.History) < 10 || len(result.VerifiedRepairs) < 10); offset += 200 {
			page, err := workflows.ListTestWorkflows(200, offset)
			if err != nil {
				return internalError(ctx, err)
			}
			for _, workflow := range page {
				if workflow.ProfileID != profile.ID || workflow.ProjectID != profile.ProjectID {
					continue
				}
				if len(result.History) < 10 {
					result.History = append(result.History, history.Summary(workflow))
				}
				if len(result.VerifiedRepairs) < 10 && workflow.Verification != nil && workflow.Verification.Status == "RESOLVED" && workflow.During != nil && workflow.After != nil &&
					workflow.Verification.BeforeWindowID == workflow.During.ID && workflow.Verification.AfterWindowID == workflow.After.ID &&
					workflow.During.ProfileID == profile.ID && workflow.After.ProfileID == profile.ID && workflow.During.DeviceID == workflow.After.DeviceID && workflow.During.DeviceID != "" && workflow.During.Source == workflow.After.Source {
					verificationSource, sourceOK := passport.SourceKind(workflow.After.Source)
					if !sourceOK {
						continue
					}
					actions := make([]string, 0, len(workflow.UserActions))
					for _, action := range workflow.UserActions {
						if action.TimestampMS > 0 && action.TimestampMS <= workflow.Verification.TimestampMS {
							actions = append(actions, action.Description)
						}
					}
					if len(actions) > 0 {
						result.VerifiedRepairs = append(result.VerifiedRepairs, domain.PassportRepair{WorkflowID: workflow.ID, Source: verificationSource, DeviceID: workflow.After.DeviceID, VerifiedAtMS: workflow.Verification.TimestampMS, Summary: workflow.Verification.Summary, UserReportedActions: actions})
					}
				}
			}
			if len(page) < 200 {
				break
			}
		}
	}
	return ctx.JSON(result)
}
