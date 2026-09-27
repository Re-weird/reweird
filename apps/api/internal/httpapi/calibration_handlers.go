package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
)

// physicalCalibrationContext decides whether a profile may ever receive a
// physical Known Good, and from which moment captures count. Only a
// project-backed profile whose probe placement the user confirmed qualifies;
// the built-in demo profile is simulator-only by design.
func (controller *Controller) physicalCalibrationContext(profile domain.ProjectProfile) (allowed bool, reason string, notBeforeMS int64, err error) {
	project, err := controller.repository.GetProject(profile.ProjectID)
	if err != nil {
		return false, "", 0, err
	}
	if project == nil {
		return false, "This profile is not backed by a project. The built-in demo profile is simulator-only; create a project profile that describes the physical circuit before calibrating it.", 0, nil
	}
	if project.ProbePlan == nil || project.ProbePlan.ProfileID != profile.ID || !project.ProbePlan.Connected || project.ProbePlan.ConnectedAtMS <= 0 {
		return false, "Confirm the generated probe plan (probe placement) before physical calibration.", 0, nil
	}
	return true, "", project.ProbePlan.ConnectedAtMS, nil
}

func (controller *Controller) calibration(ctx *fiber.Ctx) error {
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
		return apiError(ctx, fiber.StatusConflict, "PROFILE_UNCONFIRMED", "Confirm the Project Profile before calibrating.")
	}
	allowed, reason, notBefore, err := controller.physicalCalibrationContext(*profile)
	if err != nil {
		return internalError(ctx, err)
	}
	windows, err := measurements.QueryMeasurements(domain.MeasurementQuery{ProfileID: profile.ID, Source: "serial", Limit: 200})
	if err != nil {
		return internalError(ctx, err)
	}
	input := passport.CalibrationInput{
		Profile: *profile, ActiveSource: controller.source.Name(), Windows: windows,
		PhysicalAllowed: allowed, PhysicalBlock: reason, NotBeforeMS: notBefore,
	}
	deviceID := ""
	var newest int64
	for _, window := range windows {
		if window.ID > newest {
			newest, deviceID = window.ID, window.DeviceID
		}
	}
	if deviceID != "" {
		input.KnownGood, err = repository.LatestKnownGood(profile.ID, domain.BaselinePhysical, deviceID)
	} else {
		input.KnownGood, err = repository.LatestKnownGoodOfSource(profile.ID, domain.BaselinePhysical)
	}
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(passport.ComputeCalibration(input))
}

type calibrationRefusal struct {
	status int
	code   string
	detail string
}

// learnPhysicalKnownGood gathers the consecutive physical run ending at the
// selected capture and learns a baseline from it. Every window must have been
// captured after probe confirmation.
func (controller *Controller) learnPhysicalKnownGood(profile domain.ProjectProfile, window domain.MeasurementWindow, notBeforeMS int64, note string) (domain.KnownGoodBaseline, *calibrationRefusal, error) {
	measurements, ok := controller.repository.(domain.MeasurementRepository)
	if !ok {
		return domain.KnownGoodBaseline{}, &calibrationRefusal{fiber.StatusNotImplemented, "MEASUREMENT_STORAGE_UNAVAILABLE", "Measurement storage is unavailable."}, nil
	}
	windows, err := measurements.QueryMeasurements(domain.MeasurementQuery{ProfileID: profile.ID, DeviceID: window.DeviceID, Source: "serial", Limit: 200})
	if err != nil {
		return domain.KnownGoodBaseline{}, nil, err
	}
	eligible := windows[:0:0]
	for _, item := range windows {
		if item.IngestedAtMS >= notBeforeMS {
			eligible = append(eligible, item)
		}
	}
	run := passport.ConsecutiveRun(profile, eligible, window.DeviceID, window.ID, passport.CalibrationWindows)
	if len(run) == 0 || run[len(run)-1].ID != window.ID {
		return domain.KnownGoodBaseline{}, &calibrationRefusal{fiber.StatusConflict, "CAPTURE_NOT_CALIBRATABLE", "That capture is not part of an observation under the active profile revision."}, nil
	}
	record, err := passport.LearnKnownGood(profile, run, note)
	if err != nil {
		return domain.KnownGoodBaseline{}, &calibrationRefusal{fiber.StatusUnprocessableEntity, "CAPTURE_NOT_HEALTHY", err.Error()}, nil
	}
	return record, nil, nil
}
