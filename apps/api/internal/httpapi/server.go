package httpapi

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/reports"
)

type Controller struct {
	mu            sync.RWMutex
	testMu        sync.Mutex
	stage         domain.Stage
	reference     *domain.AnalysisResult
	engine        *diagnostics.Engine
	repository    domain.Repository
	source        domain.TelemetrySource
	profileID     string
	understanding *projectunderstanding.Service
	uploadRoot    string
}

type ProjectServices struct {
	Understanding *projectunderstanding.Service
	UploadRoot    string
}

func NewApp(engine *diagnostics.Engine, repository domain.Repository, source domain.TelemetrySource, profileID string, projectServices ProjectServices) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "ReWeird API",
		DisableStartupMessage: true,
		BodyLimit:             6 * 1024 * 1024,
	})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://127.0.0.1:3000",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET,POST,PUT,OPTIONS",
	}))

	controller := &Controller{
		stage:         domain.StageDiagnose,
		engine:        engine,
		repository:    repository,
		source:        source,
		profileID:     profileID,
		understanding: projectServices.Understanding,
		uploadRoot:    projectServices.UploadRoot,
	}
	if scenario, ok := source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(domain.StageDiagnose)
	}

	app.Get("/health", func(ctx *fiber.Ctx) error {
		return controller.systemStatus(ctx)
	})

	api := app.Group("/api/v1")
	api.Get("/ws/telemetry", telemetryWebSocketUpgrade, controller.telemetryWebSocket())
	api.Get("/session", controller.current)
	api.Get("/status", controller.systemStatus)
	api.Get("/report", controller.report)
	api.Get("/telemetry/status", controller.telemetryStatus)
	api.Get("/measurements", controller.listMeasurements)
	api.Get("/computer/status", controller.computerStatus)
	api.Get("/computer/scenarios", controller.computerScenarios)
	api.Post("/computer/simulate", controller.computerSimulate)
	api.Post("/computer/collect", controller.computerCollect)
	api.Get("/git/status", controller.gitStatus)
	api.Get("/git/preview/:id", controller.gitPreview)
	api.Post("/git/commit/:id", controller.gitCommit)
	api.Get("/history", controller.listHistory)
	api.Get("/history/:id", controller.historyDetail)
	api.Get("/reports/:id", controller.detailedReport)
	api.Get("/tests/recommendation", controller.recommendTest)
	api.Get("/tests/current", controller.currentTest)
	api.Post("/tests", controller.createTest)
	api.Get("/tests/:id", controller.getTest)
	api.Post("/tests/:id/start", controller.startTest)
	api.Post("/tests/:id/capture", controller.captureTest)
	api.Post("/tests/:id/remeasure", controller.remeasureTest)
	api.Post("/tests/:id/cancel", controller.cancelTest)
	api.Post("/tests/:id/actions", controller.recordTestAction)
	api.Get("/simulator/scenarios", controller.listSimulatorScenarios)
	api.Post("/simulator/scenario", controller.selectSimulatorScenario)
	api.Get("/profiles", controller.listProfiles)
	api.Get("/profiles/:id", controller.getProfile)
	api.Post("/profiles", controller.saveProfile)
	api.Put("/profiles/:id", controller.saveProfile)
	api.Post("/projects", controller.createProject)
	api.Get("/projects", controller.listProjects)
	api.Get("/projects/:id", controller.getProject)
	api.Post("/projects/:id/media", controller.uploadProjectImage)
	api.Post("/projects/:id/code", controller.uploadProjectCode)
	api.Post("/projects/:id/analyze", controller.analyzeProject)
	api.Get("/projects/:id/profile", controller.getProjectProfile)
	api.Put("/projects/:id/profile", controller.updateProjectProfile)
	api.Post("/projects/:id/profile/confirm", controller.confirmProjectProfile)
	api.Put("/projects/:id/visibility", controller.updateProjectVisibility)
	api.Get("/projects/:id/probe-plan", controller.getProbePlan)
	api.Post("/projects/:id/probe-plan/confirm", controller.confirmProbePlan)

	// Compatibility routes preserve the original dashboard and hackathon demo.
	api.Get("/demo/session", controller.current)
	api.Get("/demo/probe-plan", controller.demoProbePlan)
	api.Post("/demo/reset", controller.transition(domain.StageDiagnose))
	api.Post("/demo/wiggle", controller.transition(domain.StageTest))
	api.Post("/demo/repair", controller.transition(domain.StageVerify))

	api.Post("/patch", func(ctx *fiber.Ctx) error {
		return ctx.Status(fiber.StatusLocked).JSON(fiber.Map{
			"error":  "PATCH_LOCKED",
			"detail": "Active electrical output is disabled while passive sensing is under development.",
		})
	})

	return app
}

func (controller *Controller) current(ctx *fiber.Ctx) error {
	session, err := controller.analyze(ctx.Context())
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	return ctx.JSON(session)
}

func (controller *Controller) report(ctx *fiber.Ctx) error {
	session, err := controller.analyze(ctx.Context())
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	return ctx.JSON(reports.BuildReport(session))
}

func (controller *Controller) transition(stage domain.Stage) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if err := controller.prepareTransition(ctx.Context(), stage); err != nil {
			return serviceUnavailable(ctx, err)
		}
		session, err := controller.analyze(ctx.Context())
		if err != nil {
			return serviceUnavailable(ctx, err)
		}
		if err := controller.repository.SaveSession(session); err != nil {
			log.Printf("store diagnostic snapshot: %v", err)
		}
		return ctx.JSON(session)
	}
}

func (controller *Controller) prepareTransition(ctx context.Context, stage domain.Stage) error {
	currentEnvelope, currentErr := controller.source.Latest(ctx)
	controller.mu.RLock()
	referenceMissing := controller.reference == nil
	controller.mu.RUnlock()

	if referenceMissing && stage != domain.StageDiagnose {
		if scenario, ok := controller.source.(domain.ScenarioTelemetrySource); ok {
			scenario.SetStage(domain.StageDiagnose)
		}
		initial, err := controller.analyzeAt(ctx, domain.StageDiagnose, nil)
		if err != nil {
			return err
		}
		controller.mu.Lock()
		copyOfAnalysis := initial.Analysis
		controller.reference = &copyOfAnalysis
		controller.mu.Unlock()
	}

	controller.mu.Lock()
	controller.stage = stage
	if stage == domain.StageDiagnose {
		controller.reference = nil
	}
	controller.mu.Unlock()
	if scenario, ok := controller.source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(stage)
	}
	if waiter, ok := controller.source.(domain.FreshTelemetrySource); ok && currentErr == nil {
		waitContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := waiter.WaitNext(waitContext, currentEnvelope.Sequence); err != nil {
			return err
		}
	}
	return nil
}

func (controller *Controller) analyze(ctx context.Context) (domain.Session, error) {
	controller.mu.RLock()
	stage := controller.stage
	reference := cloneAnalysis(controller.reference)
	controller.mu.RUnlock()

	session, err := controller.analyzeAt(ctx, stage, reference)
	if err != nil {
		return domain.Session{}, err
	}
	if stage == domain.StageDiagnose && reference == nil {
		controller.mu.Lock()
		copyOfAnalysis := session.Analysis
		controller.reference = &copyOfAnalysis
		controller.mu.Unlock()
	}
	return session, nil
}

func (controller *Controller) analyzeAt(ctx context.Context, stage domain.Stage, reference *domain.AnalysisResult) (domain.Session, error) {
	controller.mu.RLock()
	profileID := controller.profileID
	controller.mu.RUnlock()
	profile, err := controller.repository.GetProfile(profileID)
	if err != nil {
		return domain.Session{}, err
	}
	if profile == nil {
		return domain.Session{}, errors.New("active Project Profile was not found")
	}
	envelope, err := controller.source.Latest(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	session, err := controller.engine.AnalyzeEnvelope(ctx, *profile, stage, controller.source.Name(), envelope, reference)
	if err != nil {
		return domain.Session{}, err
	}
	session.RawTelemetry = envelope
	if scenario, ok := controller.source.(domain.FaultScenarioSource); ok {
		session.ScenarioID = scenario.CurrentScenario()
	}
	if measurements, ok := controller.repository.(domain.MeasurementRepository); ok {
		stored, err := measurements.SaveMeasurement(domain.MeasurementWindow{
			ProfileID: envelope.ProfileID, Source: controller.source.Name(), DeviceID: envelope.DeviceID,
			Sequence: envelope.Sequence, CapturedAtMS: envelope.CapturedAtMS, Raw: envelope, Analysis: session.Analysis,
		})
		if err != nil {
			return domain.Session{}, err
		}
		session.MeasurementID = stored.ID
	}
	return session, nil
}

func (controller *Controller) telemetryStatus(ctx *fiber.Ctx) error {
	envelope, err := controller.source.Latest(ctx.Context())
	if err != nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"mode":      controller.source.Name(),
			"connected": false,
			"error":     err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{
		"mode":           controller.source.Name(),
		"connected":      true,
		"device_id":      envelope.DeviceID,
		"profile_id":     envelope.ProfileID,
		"schema_version": envelope.SchemaVersion,
		"sequence":       envelope.Sequence,
		"captured_at_ms": envelope.CapturedAtMS,
	})
}

func (controller *Controller) listMeasurements(ctx *fiber.Ctx) error {
	repository, ok := controller.repository.(domain.MeasurementRepository)
	if !ok {
		return ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "MEASUREMENT_STORAGE_UNAVAILABLE"})
	}
	profileID := ctx.Query("profile_id")
	if profileID == "" {
		controller.mu.RLock()
		profileID = controller.profileID
		controller.mu.RUnlock()
	}
	limit, err := strconv.Atoi(ctx.Query("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_LIMIT", "detail": "limit must be between 1 and 200"})
	}
	windows, err := repository.ListMeasurements(profileID, limit)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(windows)
}

func (controller *Controller) listSimulatorScenarios(ctx *fiber.Ctx) error {
	source, ok := controller.source.(domain.FaultScenarioSource)
	if !ok {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "SIMULATOR_UNAVAILABLE", "detail": "The active telemetry source is not the simulator."})
	}
	return ctx.JSON(fiber.Map{"active": source.CurrentScenario(), "scenarios": source.Scenarios()})
}

func (controller *Controller) selectSimulatorScenario(ctx *fiber.Ctx) error {
	source, ok := controller.source.(domain.FaultScenarioSource)
	if !ok {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "SIMULATOR_UNAVAILABLE", "detail": "The active telemetry source is not the simulator."})
	}
	var request struct {
		ScenarioID string `json:"scenario_id"`
	}
	if err := ctx.BodyParser(&request); err != nil || request.ScenarioID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_SCENARIO", "detail": "scenario_id is required"})
	}
	if err := source.SetScenario(request.ScenarioID); err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "INVALID_SCENARIO", "detail": err.Error()})
	}
	controller.mu.Lock()
	controller.stage = domain.StageDiagnose
	controller.reference = nil
	controller.mu.Unlock()
	session, err := controller.analyze(ctx.Context())
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if err := controller.repository.SaveSession(session); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(session)
}

func (controller *Controller) listProfiles(ctx *fiber.Ctx) error {
	items, err := controller.repository.ListProfiles()
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(items)
}

func (controller *Controller) getProfile(ctx *fiber.Ctx) error {
	profile, err := controller.repository.GetProfile(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PROFILE_NOT_FOUND"})
	}
	return ctx.JSON(profile)
}

func (controller *Controller) saveProfile(ctx *fiber.Ctx) error {
	var profile domain.ProjectProfile
	if err := ctx.BodyParser(&profile); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_JSON", "detail": err.Error()})
	}
	if routeID := ctx.Params("id"); routeID != "" && routeID != profile.ID {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PROFILE_ID_MISMATCH"})
	}
	if err := profiles.Validate(profile); err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "INVALID_PROFILE", "detail": err.Error()})
	}
	if err := controller.repository.SaveProfile(profile); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProfile(profile.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

func cloneAnalysis(value *domain.AnalysisResult) *domain.AnalysisResult {
	if value == nil {
		return nil
	}
	copyOfValue := *value
	return &copyOfValue
}

func serviceUnavailable(ctx *fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "TELEMETRY_UNAVAILABLE", "detail": err.Error()})
}

func internalError(ctx *fiber.Ctx, err error) error {
	log.Printf("request failed: %v", err)
	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "INTERNAL_ERROR"})
}
