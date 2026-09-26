package httpapi

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
)

type Controller struct {
	mu         sync.RWMutex
	stage      domain.Stage
	reference  *domain.AnalysisResult
	engine     *diagnostics.Engine
	repository domain.Repository
	source     domain.TelemetrySource
	profileID  string
}

func NewApp(engine *diagnostics.Engine, repository domain.Repository, source domain.TelemetrySource, profileID string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "ReWeird API",
		DisableStartupMessage: true,
		BodyLimit:             256 * 1024,
	})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://127.0.0.1:3000",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET,POST,PUT,OPTIONS",
	}))

	controller := &Controller{
		stage:      domain.StageDiagnose,
		engine:     engine,
		repository: repository,
		source:     source,
		profileID:  profileID,
	}
	if scenario, ok := source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(domain.StageDiagnose)
	}

	app.Get("/health", func(ctx *fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"status": "ok", "service": "reweird-api", "telemetry_mode": source.Name()})
	})

	api := app.Group("/api/v1")
	api.Get("/session", controller.current)
	api.Get("/telemetry/status", controller.telemetryStatus)
	api.Get("/profiles", controller.listProfiles)
	api.Get("/profiles/:id", controller.getProfile)
	api.Post("/profiles", controller.saveProfile)
	api.Put("/profiles/:id", controller.saveProfile)

	// Compatibility routes preserve the original dashboard and hackathon demo.
	api.Get("/projects", controller.listProfiles)
	api.Get("/demo/session", controller.current)
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
	profile, err := controller.repository.GetProfile(controller.profileID)
	if err != nil {
		return domain.Session{}, err
	}
	if profile == nil {
		return domain.Session{}, errors.New("active Project Profile was not found")
	}
	return controller.engine.Analyze(ctx, *profile, stage, controller.source, reference)
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
		"schema_version": envelope.SchemaVersion,
		"sequence":       envelope.Sequence,
		"captured_at_ms": envelope.CapturedAtMS,
	})
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
