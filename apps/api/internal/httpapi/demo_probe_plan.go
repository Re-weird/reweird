package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
)

func (controller *Controller) demoProbePlan(ctx *fiber.Ctx) error {
	profile, err := controller.repository.GetProfile("ultrasonic-demo")
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil || !profile.Confirmed {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "DEMO_PROFILE_UNAVAILABLE"})
	}
	plan, _, err := projectunderstanding.GenerateProbePlan(*profile)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(plan)
}
