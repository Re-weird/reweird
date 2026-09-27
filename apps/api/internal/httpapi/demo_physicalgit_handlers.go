package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/demodata"
)

// applyPhysicalGitDemoRestoration/applyPhysicalGitDemoBreak apply a
// simulated state transition to ONLY the canonical Physical Git demo
// project -- the id is checked explicitly, so neither can ever be used as a
// generic "mutate any project's live profile" backdoor. They exist purely
// so a judge can watch the real Restore/Verify engines react to a genuine
// change without any real hardware. Neither touches a Physical Commit
// (immutable) and neither has an equivalent for real projects.
func (controller *Controller) applyPhysicalGitDemoRestoration(ctx *fiber.Ctx) error {
	if ctx.Params("id") != demodata.PhysicalGitDemoProjectID {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if err := demodata.ApplyRestoration(controller.repository); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"status": "ok", "state": "restored"})
}

func (controller *Controller) applyPhysicalGitDemoBreak(ctx *fiber.Ctx) error {
	if ctx.Params("id") != demodata.PhysicalGitDemoProjectID {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if err := demodata.ApplyBreak(controller.repository); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"status": "ok", "state": "broken"})
}
