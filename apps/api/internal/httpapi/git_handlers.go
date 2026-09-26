package httpapi

import (
	"net"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/gitsync"
	"github.com/re-weird/reweird/apps/api/internal/reports"
)

func (controller *Controller) gitStatus(ctx *fiber.Ctx) error {
	configuration := gitsync.FromEnvironment()
	return ctx.JSON(fiber.Map{"git_sync_enabled": configuration.Enabled, "auto_commit_reports": configuration.AutoCommit, "auto_push_reports": configuration.AutoPush, "repository_configured": configuration.Repo != ""})
}

func (controller *Controller) gitReport(ctx *fiber.Ctx) (reports.DetailedReport, error) {
	workflow, err := controller.historyWorkflow(ctx)
	if err != nil {
		return reports.DetailedReport{}, err
	}
	report, err := reports.BuildDetailed(*workflow)
	if err != nil {
		return reports.DetailedReport{}, internalError(ctx, err)
	}
	return report, nil
}

func (controller *Controller) gitPreview(ctx *fiber.Ctx) error {
	report, err := controller.gitReport(ctx)
	if err != nil {
		return err
	}
	preview, err := gitsync.BuildPreview(gitsync.FromEnvironment(), report)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(preview)
}

func (controller *Controller) gitCommit(ctx *fiber.Ctx) error {
	caller := net.ParseIP(ctx.IP())
	if caller == nil || !caller.IsLoopback() {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "LOCAL_GIT_ONLY"})
	}
	var request struct {
		Approve bool `json:"approve"`
		Push    bool `json:"push"`
	}
	if err := ctx.BodyParser(&request); err != nil || !request.Approve {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "EXPLICIT_APPROVAL_REQUIRED"})
	}
	report, err := controller.gitReport(ctx)
	if err != nil {
		return err
	}
	commit, err := gitsync.Commit(gitsync.FromEnvironment(), report, request.Push)
	if err != nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "GIT_SYNC_BLOCKED", "detail": err.Error()})
	}
	return ctx.JSON(fiber.Map{"commit": commit, "pushed": request.Push})
}
