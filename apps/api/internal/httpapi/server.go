package httpapi

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/githubapp"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/reports"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

type Controller struct {
	mu            sync.RWMutex
	testMu        sync.Mutex
	profileMu     sync.Mutex
	syncMu        sync.Mutex
	stage         domain.Stage
	reference     *domain.AnalysisResult
	engine        *diagnostics.Engine
	repository    domain.Repository
	source        domain.TelemetrySource
	profileID     string
	understanding *projectunderstanding.Service
	uploadRoot    string
	github        *githubapp.Client
	// product is nil when this deployment has no MongoDB configured;
	// every equipment/me/product-data handler must check for nil and fail
	// clearly (productUnavailable) rather than panic or silently no-op.
	product domain.ProductRepository
	catalog []componentcatalog.Entry
}

type ProjectServices struct {
	Understanding *projectunderstanding.Service
	UploadRoot    string
	// GitHub is nil when no GitHub App is configured.
	GitHub *githubapp.Client
	// PollInterval re-checks linked repos for new commits; 0 disables it.
	PollInterval time.Duration
	// Product and Catalog are optional: nil/empty when MongoDB is not
	// configured for this deployment. See Controller.product's comment.
	Product domain.ProductRepository
	Catalog []componentcatalog.Entry
}

func NewApp(engine *diagnostics.Engine, repository domain.Repository, source domain.TelemetrySource, profileID string, projectServices ProjectServices, authConfigured bool) *fiber.App {
	return newApp(engine, repository, source, profileID, projectServices, ownerContext(authConfigured))
}

// newApp takes the owner-identity middleware as a parameter so tests can
// substitute a fake verifier (simulating specific signed-in owners) without
// needing a real signed token. Production always goes through NewApp, which
// wires up the real ownerContext(authConfigured) middleware above.
func newApp(engine *diagnostics.Engine, repository domain.Repository, source domain.TelemetrySource, profileID string, projectServices ProjectServices, ownerMiddleware fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "ReWeird API",
		DisableStartupMessage: true,
		BodyLimit:             6 * 1024 * 1024,
	})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://127.0.0.1:3000",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))
	app.Use(ownerMiddleware)

	controller := &Controller{
		stage:         domain.StageDiagnose,
		engine:        engine,
		repository:    repository,
		source:        source,
		profileID:     profileID,
		understanding: projectServices.Understanding,
		uploadRoot:    projectServices.UploadRoot,
		github:        projectServices.GitHub,
		product:       projectServices.Product,
		catalog:       projectServices.Catalog,
	}
	if projectServices.GitHub != nil && projectServices.PollInterval > 0 {
		go controller.PollRepositories(context.Background(), projectServices.PollInterval)
	}
	if scenario, ok := source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(domain.StageDiagnose)
	}

	app.Get("/health", func(ctx *fiber.Ctx) error {
		return controller.systemStatus(ctx)
	})
	// Outside /api/v1: GitHub can't present the API bearer token, so the
	// webhook's HMAC signature is its only credential.
	app.Post("/webhooks/github", controller.githubWebhook)

	api := app.Group("/api/v1", apiAccessControl(os.Getenv("REWEIRD_API_TOKEN")))
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
	api.Put("/projects/:id/camera-config", controller.saveCameraConfig)
	api.Delete("/projects/:id/camera-config", controller.clearCameraConfig)
	api.Post("/projects/:id/camera/test", controller.testCameraConnection)
	api.Post("/projects/:id/camera/capture-test-frame", controller.captureCameraTestFrame)
	api.Post("/projects/:id/sync", controller.syncProjectRepository)
	api.Get("/github/status", controller.githubStatus)
	api.Post("/github/connect", controller.githubConnect)
	api.Post("/github/disconnect", controller.githubDisconnect)
	api.Get("/github/repos", controller.githubRepositories)
	api.Get("/projects/:id/probe-plan", controller.getProbePlan)
	api.Post("/projects/:id/probe-plan/confirm", controller.confirmProbePlan)
	api.Post("/projects/:id/physical-commits", controller.createPhysicalCommit)
	api.Get("/projects/:id/physical-commits", controller.listPhysicalCommits)
	// diff must be registered before :commitId -- gofiber v2 matches routes
	// in registration order within the same path-segment depth, so a
	// request to .../physical-commits/diff would otherwise bind
	// commitId="diff" and 400 instead of reaching diffPhysicalCommits.
	api.Get("/projects/:id/physical-commits/diff", controller.diffPhysicalCommits)
	api.Get("/projects/:id/physical-commits/:commitId", controller.getPhysicalCommit)
	api.Get("/projects/:id/physical-commits/:commitId/detail", controller.getPhysicalCommitDetail)
	api.Post("/projects/:id/physical-commits/:commitId/analyze-hardware", controller.analyzePhysicalCommitHardware)
	api.Get("/projects/:id/physical-commits/:commitId/vision-analysis", controller.getPhysicalCommitVisionAnalysis)
	api.Get("/projects/:id/physical-commits/:commitId/restore", controller.restorePhysicalCommit)
	api.Get("/projects/:id/physical-commits/:commitId/verify", controller.verifyPhysicalCommitRestoration)
	// Demo-only, hard-gated to the canonical Physical Git demo project id --
	// see demo_physicalgit_handlers.go. No equivalent exists for real projects.
	api.Post("/projects/:id/demo/apply-restoration", controller.applyPhysicalGitDemoRestoration)
	api.Post("/projects/:id/demo/apply-break", controller.applyPhysicalGitDemoBreak)
	api.Get("/profiles/:id/passport", controller.devicePassport)
	api.Post("/profiles/:id/known-good", controller.saveKnownGood)

	api.Get("/me", controller.me)
	api.Get("/catalog", controller.listCatalog)
	api.Get("/catalog/:id", controller.getCatalogEntry)
	api.Post("/equipment", controller.createEquipment)
	api.Get("/equipment", controller.listEquipment)
	api.Get("/equipment/:id", controller.getEquipment)
	api.Patch("/equipment/:id", controller.updateEquipment)
	api.Delete("/equipment/:id", controller.deleteEquipment)
	api.Get("/projects/:id/equipment", controller.listProjectEquipment)
	api.Post("/projects/:id/equipment", controller.attachProjectEquipment)
	api.Delete("/projects/:id/equipment/:equipmentId", controller.detachProjectEquipment)

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
	currentProfile, err := controller.profileWithKnownGood(*profile, envelope)
	if err != nil {
		return domain.Session{}, err
	}
	session, err := controller.engine.AnalyzeEnvelope(ctx, currentProfile, stage, controller.source.Name(), envelope, reference)
	if err != nil {
		return domain.Session{}, err
	}
	session.RawTelemetry = envelope
	if scenario, ok := controller.source.(domain.FaultScenarioSource); ok {
		session.ScenarioID = scenario.CurrentScenario()
	}
	if measurements, ok := controller.repository.(domain.MeasurementRepository); ok {
		window := domain.MeasurementWindow{
			ProfileID: envelope.ProfileID, Source: controller.source.Name(), DeviceID: envelope.DeviceID,
			Sequence: envelope.Sequence, CapturedAtMS: envelope.CapturedAtMS, Raw: envelope, Analysis: session.Analysis,
		}
		stored, err := measurements.SaveMeasurement(window)
		if err != nil {
			return domain.Session{}, err
		}
		session.MeasurementID = stored.ID
	}
	return session, nil
}

func (controller *Controller) profileWithKnownGood(profile domain.ProjectProfile, envelope domain.TelemetryEnvelope) (domain.ProjectProfile, error) {
	source, ok := passport.SourceKind(controller.source.Name())
	if !ok {
		return profile, nil
	}
	var baseline *domain.KnownGoodBaseline
	if repository, ok := controller.repository.(domain.PassportRepository); ok && envelope.ProfileID == profile.ID {
		stored, err := repository.LatestKnownGood(profile.ID, source, envelope.DeviceID)
		if err != nil {
			return domain.ProjectProfile{}, err
		}
		baseline = stored
	}
	return passport.ApplyKnownGood(profile, source, baseline), nil
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

// listMeasurements answers "give me recent/scoped diagnostic windows" --
// profile_id/limit always worked; probe/device_id/source/since_ms/until_ms
// are additional optional filters over the same measurement_windows table
// and the same response shape, so there is exactly one measurement API
// rather than a second one for scoped queries.
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

	query := domain.MeasurementQuery{
		ProfileID: profileID, DeviceID: ctx.Query("device_id"), Source: ctx.Query("source"),
		Probe: ctx.Query("probe"), Limit: limit,
	}
	if raw := ctx.Query("since_ms"); raw != "" {
		since, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_SINCE_MS", "since_ms must be a Unix millisecond timestamp.")
		}
		query.SinceMS = &since
	}
	if raw := ctx.Query("until_ms"); raw != "" {
		until, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_UNTIL_MS", "until_ms must be a Unix millisecond timestamp.")
		}
		query.UntilMS = &until
	}

	windows, err := repository.QueryMeasurements(query)
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
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	var profile domain.ProjectProfile
	if err := ctx.BodyParser(&profile); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_JSON", "detail": err.Error()})
	}
	if routeID := ctx.Params("id"); routeID != "" && routeID != profile.ID {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PROFILE_ID_MISMATCH"})
	}
	// This legacy endpoint is draft-only. Project profiles must use the
	// project-specific correction and confirmation workflow so clients cannot
	// bypass conflict resolution or manufacture trusted probe mappings.
	if profile.Confirmed || profile.ConfirmedAtMS != 0 || profile.ConfirmedBy != "" ||
		strings.EqualFold(profile.AnalysisStatus, "CONFIRMED") || len(profile.Probes) != 0 {
		return apiError(ctx, fiber.StatusConflict, "CONFIRMATION_REQUIRED", "Confirmation and probe configuration are server-controlled; use the project confirmation workflow.")
	}
	for _, component := range profile.Components {
		if component.Confirmed {
			return apiError(ctx, fiber.StatusConflict, "CONFIRMATION_REQUIRED", "Component confirmation is server-controlled.")
		}
	}
	for _, connection := range profile.Connections {
		if connection.Confirmed {
			return apiError(ctx, fiber.StatusConflict, "CONFIRMATION_REQUIRED", "Connection confirmation is server-controlled.")
		}
	}
	existing, err := controller.repository.GetProfile(profile.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if existing != nil && existing.Confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_CONFIRMED", "Confirmed profiles cannot be changed through draft routes.")
	}
	project, err := controller.repository.GetProject(profile.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if project != nil || profile.ProjectID != "" && profile.ProjectID != profile.ID {
		return apiError(ctx, fiber.StatusConflict, "PROJECT_PROFILE_WORKFLOW_REQUIRED", "Use the project profile correction workflow for project-backed profiles.")
	}
	profile.Confirmed = false
	profile.ConfirmedBy = ""
	profile.ConfirmedAtMS = 0
	profile.Probes = nil
	profile.AnalysisStatus = "DRAFT"
	profile.ProjectID = ""
	if existing != nil {
		profile.CreatedAtMS = existing.CreatedAtMS
		profile.Version = existing.Version + 1
	} else {
		profile.CreatedAtMS = 0
		profile.Version = 1
	}
	profile.UpdatedAtMS = 0
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
