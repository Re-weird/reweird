package httpapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/telemetrystore"
)

// queryTelemetry answers "what happened electrically over time?" from
// Tiger Data -- a different question, and a different response shape,
// from GET /api/v1/measurements (whole SQLite-backed diagnostic windows).
// It is read-only and scoped/bounded: probe, profile_id, a time range, and
// a limit that is always clamped, so this can never become an unbounded
// scan.
func (controller *Controller) queryTelemetry(ctx *fiber.Ctx) error {
	if controller.telemetry == nil {
		return apiError(ctx, fiber.StatusServiceUnavailable, "TELEMETRY_STORE_UNAVAILABLE", "This deployment has no Tiger Data configured (TIGER_DATABASE_URL); scoped telemetry history is unavailable.")
	}
	query := domain.TelemetryQuery{
		Probe:     ctx.Query("probe"),
		ProfileID: ctx.Query("profile_id"),
		Limit:     telemetrystore.DefaultQueryLimit,
	}
	if raw := ctx.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > telemetrystore.MaxQueryLimit {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer between 1 and "+strconv.Itoa(telemetrystore.MaxQueryLimit)+".")
		}
		query.Limit = limit
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
	records, err := controller.telemetry.Query(ctx.Context(), query)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(records)
}
