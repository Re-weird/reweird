package httpapi

import (
	"os"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/gitsync"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

func (controller *Controller) systemStatus(ctx *fiber.Ctx) error {
	database := "unknown"
	if pinger, ok := controller.repository.(interface{ Ping() error }); ok {
		if err := pinger.Ping(); err == nil {
			database = "ok"
		} else {
			database = "error"
		}
	}
	mode := controller.source.Name()
	telemetry := "unavailable"
	if _, err := controller.source.Latest(ctx.Context()); err == nil {
		telemetry = "available"
	}
	gemini := "not_configured"
	if os.Getenv("GEMINI_API_KEY") != "" {
		gemini = "configured_vision_only"
	}
	computer := "not_running"
	if computerEnabled() {
		if runtime.GOOS == "windows" {
			computer = "available_on_request"
		} else {
			computer = "unsupported_platform"
		}
	}
	git := gitsync.FromEnvironment()
	response := fiber.Map{"api": "ok", "database": database, "telemetry_mode": mode, "telemetry": telemetry, "esp32": mode == "serial" && telemetry == "available", "gemini": gemini, "patch": "locked", "git_sync_enabled": git.Enabled, "git_repository_configured": git.Repo != "", "computer_agent": computer, "report_retention": "persisted_in_database", "measurement_window_limit": store.MeasurementWindowLimit()}
	if database == "error" {
		response["api"] = "degraded"
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(response)
	}
	return ctx.JSON(response)
}
