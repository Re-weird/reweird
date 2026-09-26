package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
)

func (controller *Controller) testRepositories(ctx *fiber.Ctx) (domain.TestWorkflowRepository, domain.MeasurementRepository, error) {
	workflows, ok := controller.repository.(domain.TestWorkflowRepository)
	if !ok {
		return nil, nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "TEST_STORAGE_UNAVAILABLE"})
	}
	measurements, ok := controller.repository.(domain.MeasurementRepository)
	if !ok {
		return nil, nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "MEASUREMENT_STORAGE_UNAVAILABLE"})
	}
	return workflows, measurements, nil
}

func (controller *Controller) activeTestProfile() (*domain.ProjectProfile, error) {
	controller.mu.RLock()
	id := controller.profileID
	controller.mu.RUnlock()
	profile, err := controller.repository.GetProfile(id)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, errors.New("active Project Profile was not found")
	}
	return profile, nil
}

func (controller *Controller) activeTestProfileWithKnownGood() (*domain.ProjectProfile, error) {
	profile, err := controller.activeTestProfile()
	if err != nil {
		return nil, err
	}
	envelope, err := controller.source.Latest(context.Background())
	if err != nil {
		return profile, nil
	}
	current, err := controller.profileWithKnownGood(*profile, envelope)
	if err != nil {
		return nil, err
	}
	return &current, nil
}

func (controller *Controller) recommendTest(ctx *fiber.Ctx) error {
	profile, err := controller.activeTestProfileWithKnownGood()
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if !profile.Confirmed {
		return testConflict(ctx, "PROFILE_UNCONFIRMED", "Confirm the Project Profile before planning a test.")
	}
	session, err := controller.analyze(ctx.Context())
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	return ctx.JSON(testplanner.New().Recommend(*profile, session.Analysis, session.ID))
}

func (controller *Controller) currentTest(ctx *fiber.Ctx) error {
	workflows, _, err := controller.testRepositories(ctx)
	if err != nil {
		return err
	}
	profile, err := controller.activeTestProfile()
	if err != nil {
		return internalError(ctx, err)
	}
	workflow, err := workflows.LatestTestWorkflow(profile.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if workflow == nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "TEST_NOT_FOUND"})
	}
	return ctx.JSON(workflow)
}

func (controller *Controller) getTest(ctx *fiber.Ctx) error {
	workflows, _, err := controller.testRepositories(ctx)
	if err != nil {
		return err
	}
	workflow, err := workflows.GetTestWorkflow(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if workflow == nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "TEST_NOT_FOUND"})
	}
	return ctx.JSON(workflow)
}

func (controller *Controller) createTest(ctx *fiber.Ctx) error {
	controller.testMu.Lock()
	defer controller.testMu.Unlock()
	workflows, _, err := controller.testRepositories(ctx)
	if err != nil {
		return err
	}
	profile, err := controller.activeTestProfileWithKnownGood()
	if err != nil {
		return internalError(ctx, err)
	}
	var recommendation domain.TestRecommendation
	if err := ctx.BodyParser(&recommendation); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_JSON", "detail": err.Error()})
	}
	id, err := randomTestID()
	if err != nil {
		return internalError(ctx, err)
	}
	if recommendation.ID == "" {
		recommendation.ID = id
	}
	plan, err := testplanner.New().Plan(*profile, recommendation)
	if err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "INVALID_TEST_RECOMMENDATION", "detail": err.Error()})
	}
	plan.ID = id
	now := time.Now().UTC().UnixMilli()
	workflow := domain.DiagnosticWorkflow{
		ID: id, SessionID: recommendation.SessionID, ProjectID: profile.ProjectID, ProfileID: profile.ID, ProfileVersion: profile.Version, ProfileSnapshot: profile,
		Status: domain.TestPlanned, Plan: plan, CreatedAtMS: now, UpdatedAtMS: now,
	}
	if scenario, ok := controller.source.(domain.FaultScenarioSource); ok {
		workflow.ScenarioID = scenario.CurrentScenario()
	}
	if plan.RequiresPatch {
		workflow.Status = domain.TestLocked
	}
	if err := workflows.SaveTestWorkflow(workflow); err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(workflow)
}

func (controller *Controller) startTest(ctx *fiber.Ctx) error {
	return controller.advanceTest(ctx, "start")
}
func (controller *Controller) captureTest(ctx *fiber.Ctx) error {
	return controller.advanceTest(ctx, "capture")
}
func (controller *Controller) remeasureTest(ctx *fiber.Ctx) error {
	return controller.advanceTest(ctx, "remeasure")
}
func (controller *Controller) cancelTest(ctx *fiber.Ctx) error {
	return controller.advanceTest(ctx, "cancel")
}

func (controller *Controller) advanceTest(ctx *fiber.Ctx, action string) error {
	controller.testMu.Lock()
	defer controller.testMu.Unlock()
	workflows, measurements, err := controller.testRepositories(ctx)
	if err != nil {
		return err
	}
	workflow, err := workflows.GetTestWorkflow(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if workflow == nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "TEST_NOT_FOUND"})
	}
	profile, err := controller.activeTestProfileWithKnownGood()
	if err != nil {
		return internalError(ctx, err)
	}
	if workflow.ProfileID != profile.ID {
		return testConflict(ctx, "PROFILE_CHANGED", "The active Project Profile differs from this test.")
	}
	if workflow.ProfileVersion != profile.Version {
		return testConflict(ctx, "PROFILE_CHANGED", "The Project Profile changed after this test was planned. Create a new plan against the current limits.")
	}
	if workflow.Plan.RequiresPatch || workflow.Status == domain.TestLocked {
		return ctx.Status(fiber.StatusLocked).JSON(fiber.Map{"error": "PATCH_LOCKED", "detail": workflow.Plan.Unavailable})
	}
	allowed := false
	switch action {
	case "start":
		allowed = workflow.Status == domain.TestPlanned
	case "capture":
		allowed = workflow.Status == domain.TestWaitingForUser || workflow.Status == domain.TestReady
	case "remeasure":
		allowed = workflow.Status == domain.TestCompleted || workflow.Status == domain.TestInconclusive || workflow.Status == domain.TestUnresolved
	case "cancel":
		allowed = workflow.Status == domain.TestPlanned || workflow.Status == domain.TestWaitingForUser || workflow.Status == domain.TestReady || workflow.Status == domain.TestInconclusive
	}
	if !allowed {
		return testConflict(ctx, "INVALID_TEST_TRANSITION", fmt.Sprintf("Cannot %s a test in state %s.", action, workflow.Status))
	}
	if action == "cancel" {
		workflow.Status = domain.TestCancelled
		if err := saveTestState(workflows, workflow); err != nil {
			return internalError(ctx, err)
		}
		return ctx.JSON(workflow)
	}
	if scenario, ok := controller.source.(domain.FaultScenarioSource); ok && workflow.ScenarioID != "" {
		if err := scenario.SetScenario(workflow.ScenarioID); err != nil {
			return internalError(ctx, err)
		}
	}
	stage := domain.StageDiagnose
	switch action {
	case "start":
		workflow.Status = domain.TestCapturingBaseline
	case "capture":
		workflow.Status = domain.TestCapturingTest
		if workflow.Plan.Recommendation.TestType == domain.TestMovementCorrelation {
			stage = domain.StageTest
		}
	case "remeasure":
		workflow.Status = domain.TestVerifying
		if _, simulated := controller.source.(domain.FaultScenarioSource); simulated {
			stage = domain.StageVerify
		}
	}
	if scenario, ok := controller.source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(stage)
	}
	if err := saveTestState(workflows, workflow); err != nil {
		return internalError(ctx, err)
	}
	var previous *domain.MeasurementWindow
	if workflow.During != nil {
		previous = workflow.During
	} else if workflow.Baseline != nil {
		previous = workflow.Baseline
	}
	window, err := controller.captureTestWindow(ctx.Context(), measurements, *profile, previous)
	if err != nil {
		workflow.Status, workflow.Error = domain.TestFailed, err.Error()
		_ = saveTestState(workflows, workflow)
		return serviceUnavailable(ctx, err)
	}
	switch action {
	case "start":
		workflow.Baseline = &window
		if workflow.Plan.Recommendation.RequiresUserAction {
			workflow.Status = domain.TestWaitingForUser
		} else {
			workflow.Status = domain.TestReady
		}
	case "capture":
		workflow.During = &window
		workflow.Status = domain.TestAnalyzing
		if err := saveTestState(workflows, workflow); err != nil {
			return internalError(ctx, err)
		}
		result := testplanner.New().Evaluate(*profile, workflow.Plan, *workflow.Baseline, window)
		workflow.Result = &result
		if result.Result == "INCONCLUSIVE" {
			workflow.Status = domain.TestInconclusive
		} else {
			workflow.Status = domain.TestCompleted
		}
	case "remeasure":
		workflow.After = &window
		if workflow.During == nil {
			return testConflict(ctx, "MISSING_TEST_WINDOW", "Capture the test window before verifying.")
		}
		verification := testplanner.New().Verify(*profile, workflow.Plan.Recommendation.TargetProbes, *workflow.During, window)
		workflow.Verification = &verification
		if verification.Status == "RESOLVED" {
			workflow.Status = domain.TestResolved
		} else if verification.Status == "INCONCLUSIVE" {
			workflow.Status = domain.TestInconclusive
		} else {
			workflow.Status = domain.TestUnresolved
		}
	}
	if err := saveTestState(workflows, workflow); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(workflow)
}

func (controller *Controller) captureTestWindow(ctx context.Context, repository domain.MeasurementRepository, profile domain.ProjectProfile, previous *domain.MeasurementWindow) (domain.MeasurementWindow, error) {
	var envelope domain.TelemetryEnvelope
	var err error
	if fresh, ok := controller.source.(domain.FreshTelemetrySource); ok {
		lastSequence := uint64(0)
		if previous != nil {
			lastSequence = previous.Sequence
		}
		current, currentErr := fresh.Latest(ctx)
		if currentErr == nil && previous != nil && current.DeviceID == previous.DeviceID && current.Sequence < lastSequence && current.CapturedAtMS > previous.CapturedAtMS {
			// A device or API restart reset its sequence; its newer capture is still fresh.
			envelope = current
		} else {
			waitContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			envelope, err = fresh.WaitNext(waitContext, lastSequence)
		}
	} else {
		envelope, err = controller.source.Latest(ctx)
	}
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	if previous != nil && envelope.DeviceID == previous.DeviceID && envelope.Sequence <= previous.Sequence && envelope.CapturedAtMS <= previous.CapturedAtMS {
		return domain.MeasurementWindow{}, errors.New("no fresh telemetry window was received")
	}
	analysis, err := signalanalysis.New().Analyze(envelope, profile)
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	return repository.SaveMeasurement(domain.MeasurementWindow{
		ProfileID: profile.ID, Source: controller.source.Name(), DeviceID: envelope.DeviceID,
		Sequence: envelope.Sequence, CapturedAtMS: envelope.CapturedAtMS, Raw: envelope, Analysis: analysis,
	})
}

func saveTestState(repository domain.TestWorkflowRepository, workflow *domain.DiagnosticWorkflow) error {
	workflow.UpdatedAtMS = time.Now().UTC().UnixMilli()
	return repository.SaveTestWorkflow(*workflow)
}

func randomTestID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "test-" + hex.EncodeToString(raw[:]), nil
}

func testConflict(ctx *fiber.Ctx, code, detail string) error {
	return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": code, "detail": detail})
}
