package httpapi

import (
	"net"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/computer"
)

func computerEnabled() bool {
	return strings.EqualFold(os.Getenv("COMPUTER_DIAGNOSTICS_ENABLED"), "true")
}

func (controller *Controller) computerStatus(ctx *fiber.Ctx) error {
	return ctx.JSON(fiber.Map{"real_collection_enabled": computerEnabled(), "collector": computer.PlatformCollector().Name(), "simulator_available": true, "active_operations_enabled": false})
}

func (controller *Controller) computerScenarios(ctx *fiber.Ctx) error {
	return ctx.JSON(fiber.Map{"scenarios": computer.Scenarios()})
}

func (controller *Controller) computerSimulate(ctx *fiber.Ctx) error {
	var input struct {
		ScenarioID string `json:"scenario_id"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_REQUEST"})
	}
	snapshot, expected, err := computer.Simulate(input.ScenarioID)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "UNKNOWN_SCENARIO"})
	}
	analysis, err := computer.Analyze(snapshot, expected)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "COMPUTER_ANALYSIS_FAILED"})
	}
	return ctx.JSON(analysis)
}

func (controller *Controller) computerCollect(ctx *fiber.Ctx) error {
	if !computerEnabled() {
		return ctx.Status(fiber.StatusLocked).JSON(fiber.Map{"error": "COMPUTER_COLLECTION_DISABLED", "detail": "Set COMPUTER_DIAGNOSTICS_ENABLED=true on a trusted local installation to opt in."})
	}
	requester := net.ParseIP(ctx.IP())
	if requester == nil || !requester.IsLoopback() {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "LOCAL_COLLECTION_ONLY"})
	}
	var expected computer.Expectations
	if len(ctx.Body()) > 0 {
		if err := ctx.BodyParser(&expected); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_REQUEST"})
		}
	}
	if err := computer.ValidateExpectations(expected); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_EXPECTATIONS", "detail": err.Error()})
	}
	snapshot, err := computer.PlatformCollector().Collect(ctx.Context(), expected)
	if err != nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "COMPUTER_COLLECTION_UNAVAILABLE", "detail": err.Error()})
	}
	analysis, err := computer.Analyze(snapshot, expected)
	if err != nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "COMPUTER_ANALYSIS_FAILED", "detail": err.Error()})
	}
	return ctx.JSON(analysis)
}
