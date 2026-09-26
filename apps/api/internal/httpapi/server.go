package httpapi

import (
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

type DemoController struct {
	mu         sync.RWMutex
	stage      domain.Stage
	engine     *diagnostics.Engine
	repository domain.SessionRepository
}

func NewApp(engine *diagnostics.Engine, repository domain.SessionRepository) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "ReWeird API", DisableStartupMessage: true})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://127.0.0.1:3000",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET,POST,OPTIONS",
	}))

	controller := &DemoController{stage: domain.StageDiagnose, engine: engine, repository: repository}

	app.Get("/health", func(context *fiber.Ctx) error {
		return context.JSON(fiber.Map{"status": "ok", "service": "reweird-api"})
	})

	api := app.Group("/api/v1")
	api.Get("/projects", func(context *fiber.Ctx) error {
		return context.JSON([]fiber.Map{{
			"id": "ultrasonic-demo",
			"name": "Ultrasonic Distance Sensor",
			"controller": "ESP32",
			"logic_voltage": 3.3,
		}})
	})
	api.Get("/demo/session", controller.current)
	api.Post("/demo/reset", controller.transition(domain.StageDiagnose))
	api.Post("/demo/wiggle", controller.transition(domain.StageTest))
	api.Post("/demo/repair", controller.transition(domain.StageVerify))

	return app
}

func (controller *DemoController) current(context *fiber.Ctx) error {
	controller.mu.RLock()
	stage := controller.stage
	controller.mu.RUnlock()
	return context.JSON(controller.engine.Analyze(stage))
}

func (controller *DemoController) transition(stage domain.Stage) fiber.Handler {
	return func(context *fiber.Ctx) error {
		controller.mu.Lock()
		controller.stage = stage
		session := controller.engine.Analyze(stage)
		controller.mu.Unlock()
		if err := controller.repository.SaveSession(session); err != nil {
			log.Printf("store diagnostic snapshot: %v", err)
		}
		return context.JSON(session)
	}
}
