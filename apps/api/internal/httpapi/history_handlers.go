package httpapi

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/history"
	"github.com/re-weird/reweird/apps/api/internal/reports"
)

var historyIDPattern = regexp.MustCompile(`^test-[a-f0-9]{32}$`)

func (controller *Controller) historyRepository(ctx *fiber.Ctx) (domain.HistoryRepository, error) {
	repository, ok := controller.repository.(domain.HistoryRepository)
	if !ok {
		return nil, ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "HISTORY_STORAGE_UNAVAILABLE"})
	}
	return repository, nil
}

func (controller *Controller) listHistory(ctx *fiber.Ctx) error {
	repository, err := controller.historyRepository(ctx)
	if err != nil {
		return err
	}
	limit, err := strconv.Atoi(ctx.Query("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_LIMIT"})
	}
	projectID := strings.TrimSpace(ctx.Query("project_id"))
	status := domain.HistoryStatus(strings.TrimSpace(ctx.Query("status")))
	order := ctx.Query("sort", "newest")
	if order != "newest" && order != "oldest" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_SORT"})
	}
	if status != "" && !validHistoryStatus(status) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_STATUS"})
	}
	items := make([]domain.HistorySummary, 0)
	for offset := 0; offset < 2000; offset += 200 {
		page, err := repository.ListTestWorkflows(200, offset)
		if err != nil {
			return internalError(ctx, err)
		}
		for _, workflow := range page {
			project, err := controller.repository.GetProject(workflow.ProjectID)
			if err != nil {
				return internalError(ctx, err)
			}
			if project != nil && project.OwnerID != ownerID(ctx) {
				continue
			}
			summary := history.Summary(workflow)
			if projectID != "" && summary.ProjectID != projectID {
				continue
			}
			if status != "" && summary.Status != status {
				continue
			}
			items = append(items, summary)
		}
		if len(page) < 200 {
			break
		}
		if offset == 1800 {
			return ctx.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{"error": "HISTORY_SCAN_LIMIT", "detail": "History exceeds the 2000-record scan limit; use a retention/export strategy before adding more workflows."})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if order == "oldest" {
			return items[i].StartedAtMS < items[j].StartedAtMS
		}
		return items[i].StartedAtMS > items[j].StartedAtMS
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return ctx.JSON(fiber.Map{"items": items, "count": len(items)})
}

func validHistoryStatus(status domain.HistoryStatus) bool {
	switch status {
	case domain.HistoryOpen, domain.HistoryTesting, domain.HistoryWaitingForUser, domain.HistoryVerifying, domain.HistoryResolved, domain.HistoryImproved, domain.HistoryUnresolved, domain.HistoryCancelled, domain.HistoryInconclusive:
		return true
	default:
		return false
	}
}

func (controller *Controller) historyDetail(ctx *fiber.Ctx) error {
	workflow, err := controller.historyWorkflow(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(history.Detail(*workflow))
}

func (controller *Controller) historyWorkflow(ctx *fiber.Ctx) (*domain.DiagnosticWorkflow, error) {
	if !historyIDPattern.MatchString(ctx.Params("id")) {
		return nil, ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_HISTORY_ID"})
	}
	repository, err := controller.historyRepository(ctx)
	if err != nil {
		return nil, err
	}
	workflow, err := repository.GetTestWorkflow(ctx.Params("id"))
	if err != nil {
		return nil, internalError(ctx, err)
	}
	if workflow == nil {
		return nil, ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "HISTORY_NOT_FOUND"})
	}
	project, err := controller.repository.GetProject(workflow.ProjectID)
	if err != nil {
		return nil, internalError(ctx, err)
	}
	if project != nil && project.OwnerID != ownerID(ctx) {
		return nil, apiError(ctx, 404, "HISTORY_NOT_FOUND", "History not found.")
	}
	return workflow, nil
}

func (controller *Controller) detailedReport(ctx *fiber.Ctx) error {
	workflow, err := controller.historyWorkflow(ctx)
	if err != nil {
		return err
	}
	report, err := reports.BuildDetailed(*workflow)
	if err != nil {
		return internalError(ctx, err)
	}
	format := ctx.Query("format", "json")
	if format != "json" && format != "md" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_REPORT_FORMAT"})
	}
	if format == "md" {
		markdown := reports.Markdown(report)
		if len(markdown) > reports.DetailedReportLimit() {
			return ctx.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{"error": "REPORT_SIZE_LIMIT"})
		}
		ctx.Set("Content-Type", "text/markdown; charset=utf-8")
		if ctx.Query("download") == "1" {
			ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.md\"", workflow.ID))
		}
		return ctx.SendString(markdown)
	}
	data, err := reports.JSON(report)
	if err != nil {
		return internalError(ctx, err)
	}
	ctx.Set("Content-Type", "application/json; charset=utf-8")
	if ctx.Query("download") == "1" {
		ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", workflow.ID))
	}
	return ctx.Send(data)
}

func (controller *Controller) recordTestAction(ctx *fiber.Ctx) error {
	controller.testMu.Lock()
	defer controller.testMu.Unlock()
	workflow, err := controller.historyWorkflow(ctx)
	if err != nil {
		return err
	}
	if workflow.Status != domain.TestWaitingForUser && workflow.Status != domain.TestCompleted && workflow.Status != domain.TestUnresolved {
		return testConflict(ctx, "INVALID_TEST_TRANSITION", "User actions can be recorded only during a guided test or before re-measurement.")
	}
	var request struct {
		Description string `json:"description"`
	}
	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "INVALID_JSON"})
	}
	request.Description = strings.TrimSpace(request.Description)
	if request.Description == "" || len(request.Description) > 500 || strings.ContainsAny(request.Description, "\x00\r\n") {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "INVALID_USER_ACTION", "detail": "description must be 1-500 characters on one line"})
	}
	if reports.ScanSecrets(request.Description) > 0 {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "POTENTIAL_SECRET", "detail": "Remove credentials from the action note before saving it."})
	}
	identifier, err := randomTestID()
	if err != nil {
		return internalError(ctx, err)
	}
	workflow.UserActions = append(workflow.UserActions, domain.UserAction{ID: "action-" + strings.TrimPrefix(identifier, "test-"), Description: request.Description, TimestampMS: time.Now().UTC().UnixMilli()})
	if len(workflow.UserActions) > 20 {
		return ctx.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{"error": "ACTION_LIMIT"})
	}
	repository, err := controller.historyRepository(ctx)
	if err != nil {
		return err
	}
	if err := saveTestState(repository, workflow); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(workflow)
}
