package httpapi

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/cameracapture"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

type cameraSourceInput struct {
	SourceType string `json:"source_type"`
	URL        string `json:"url"`
}

// saveCameraConfig persists a project's vision camera source. It validates
// the URL shape (scheme, host, no embedded credentials) but never contacts
// the camera -- that only ever happens through Test Connection or Capture
// Test Frame, both explicit, separate actions.
func (controller *Controller) saveCameraConfig(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	var input cameraSourceInput
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Camera configuration must be valid JSON.")
	}
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" {
		sourceType = domain.CameraSourceMJPEG
	}
	if sourceType != domain.CameraSourceMJPEG {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "UNSUPPORTED_CAMERA_SOURCE", "Only an mjpeg camera source is supported.")
	}
	trimmedURL := strings.TrimSpace(input.URL)
	if _, err := cameracapture.ValidateURL(trimmedURL); err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_CAMERA_URL", err.Error())
	}
	config := domain.CameraConfig{SourceType: sourceType, URL: trimmedURL}
	if err := controller.repository.SetProjectCameraConfig(project.ID, &config); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

// clearCameraConfig removes a project's vision camera source. Physical
// Commit creation then behaves exactly as it did before this feature
// existed -- no image is auto-captured, and manual upload keeps working.
func (controller *Controller) clearCameraConfig(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if err := controller.repository.SetProjectCameraConfig(project.ID, nil); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

// resolveCameraSourceURL returns the URL to contact for a test/capture
// request: an explicit override from the request body (letting the UI test
// a URL before saving it) if present, otherwise the project's saved
// CameraConfig. Both paths are scoped to this one already-ownership-checked
// project -- there is no way to reach this code with an arbitrary caller
// target unrelated to a project the caller can already access.
func resolveCameraSourceURL(project *domain.Project, override cameraSourceInput) (string, bool) {
	if trimmed := strings.TrimSpace(override.URL); trimmed != "" {
		return trimmed, true
	}
	if project.CameraConfig != nil {
		return project.CameraConfig.URL, true
	}
	return "", false
}

// testCameraConnection performs a real connection attempt against the
// configured (or body-supplied, for testing before saving) camera source.
// It never returns CONNECTED unless a frame was actually decoded, and it
// never sends anything to Gemini.
func (controller *Controller) testCameraConnection(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	var input cameraSourceInput
	if len(ctx.Body()) > 0 {
		if err := ctx.BodyParser(&input); err != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Camera source must be valid JSON.")
		}
	}
	sourceURL, configured := resolveCameraSourceURL(project, input)
	if !configured {
		return ctx.JSON(fiber.Map{"status": cameracapture.StatusNotConfigured, "message": "No vision camera is configured for this project."})
	}
	result := cameracapture.Test(ctx.Context(), sourceURL)
	return ctx.JSON(fiber.Map{
		"status":          result.Status,
		"content_type":    result.ContentType,
		"frame_available": result.FrameAvailable,
		"latency_ms":      result.LatencyMS,
		"message":         result.Message,
	})
}

const maxTestFrameBase64Bytes = 8 * 1024 * 1024

// captureCameraTestFrame extracts one real frame and returns it inline for
// display so the user can confirm camera positioning. It never creates a
// Physical Commit, never persists the frame anywhere, and never sends it to
// Gemini.
func (controller *Controller) captureCameraTestFrame(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	var input cameraSourceInput
	if len(ctx.Body()) > 0 {
		if err := ctx.BodyParser(&input); err != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Camera source must be valid JSON.")
		}
	}
	sourceURL, configured := resolveCameraSourceURL(project, input)
	if !configured {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "CAMERA_NOT_CONFIGURED", "No vision camera is configured for this project.")
	}
	frame, err := cameracapture.Capture(ctx.Context(), sourceURL)
	if err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "CAMERA_CAPTURE_FAILED", "Camera frame could not be captured: "+err.Error())
	}
	if len(frame.Bytes) > maxTestFrameBase64Bytes {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "CAMERA_CAPTURE_FAILED", "Camera frame exceeds the preview size limit.")
	}
	return ctx.JSON(fiber.Map{
		"content_type": frame.ContentType,
		"width":        frame.Width,
		"height":       frame.Height,
		"size_bytes":   len(frame.Bytes),
		"image_base64": base64.StdEncoding.EncodeToString(frame.Bytes),
	})
}
