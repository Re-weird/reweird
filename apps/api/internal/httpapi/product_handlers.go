package httpapi

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/projects"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

// productUnavailable is returned by every equipment/me/catalog handler when
// this deployment has no MongoDB configured. It is a clear, immediate
// failure rather than a silent no-op or a fallback to fabricated data -- the
// rest of the API (SQLite-backed diagnostics, Demo Mode, the simulator)
// keeps working normally regardless of this.
func productUnavailable(ctx *fiber.Ctx) error {
	return apiError(ctx, fiber.StatusServiceUnavailable, "PRODUCT_DATA_UNAVAILABLE", "This deployment has no MongoDB configured (MONGODB_URI/MONGODB_DATABASE); accounts and equipment are unavailable.")
}

// requireOwner returns the verified caller identity, or writes a 401 and
// returns ok=false if the caller is anonymous/Demo Mode. Unlike Projects
// (which allow anonymous creation), every equipment/me endpoint requires a
// real signed-in identity -- Demo Mode must never create permanent records.
func requireOwner(ctx *fiber.Ctx) (owner string, ok bool) {
	owner = ownerID(ctx)
	if owner == "" {
		_ = apiError(ctx, fiber.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "This endpoint requires a signed-in account.")
		return "", false
	}
	return owner, true
}

// me resolves (creating on first sight) and returns the caller's own
// persistent profile. It is the one place ResolveUser is called from the
// HTTP layer, using only the server-verified email/name that travelled
// with the token itself (see ownerProfileFromContext) - never anything the
// request body claims.
func (controller *Controller) me(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	profile := ownerProfileFromContext(ctx)
	user, err := controller.product.ResolveUser(ctx.Context(), "google", owner, domain.UserProfileHints{
		Email: profile.Email, DisplayName: profile.Name,
	})
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(user)
}

func equipmentInput(ctx *fiber.Ctx) (struct {
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	Manufacturer   string   `json:"manufacturer"`
	Model          string   `json:"model"`
	SerialNumber   string   `json:"serial_number"`
	ControllerType string   `json:"controller_type"`
	LogicVoltage   *float64 `json:"logic_voltage"`
	Notes          string   `json:"notes"`
	CatalogID      *string  `json:"catalog_id"`
}, error) {
	var input struct {
		Name           string   `json:"name"`
		Category       string   `json:"category"`
		Manufacturer   string   `json:"manufacturer"`
		Model          string   `json:"model"`
		SerialNumber   string   `json:"serial_number"`
		ControllerType string   `json:"controller_type"`
		LogicVoltage   *float64 `json:"logic_voltage"`
		Notes          string   `json:"notes"`
		CatalogID      *string  `json:"catalog_id"`
	}
	err := ctx.BodyParser(&input)
	return input, err
}

// validateCatalogID confirms a supplied catalog_id exists in the trusted,
// read-only Component Catalog. It never invents or copies catalog data --
// Equipment only ever stores the id itself.
func (controller *Controller) validateCatalogID(catalogID string) bool {
	_, ok := componentcatalog.Find(controller.catalog, catalogID)
	return ok
}

func (controller *Controller) createEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	input, err := equipmentInput(ctx)
	if err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Equipment details must be valid JSON.")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_EQUIPMENT", "Equipment requires a name.")
	}
	if input.CatalogID != nil && strings.TrimSpace(*input.CatalogID) != "" {
		if !controller.validateCatalogID(*input.CatalogID) {
			return apiError(ctx, fiber.StatusUnprocessableEntity, "UNKNOWN_CATALOG_ID", "catalog_id does not match any known Component Catalog entry.")
		}
	} else {
		input.CatalogID = nil
	}
	id, err := projects.NewID(name)
	if err != nil {
		return internalError(ctx, err)
	}
	now := time.Now().UTC().UnixMilli()
	equipment := domain.Equipment{
		ID: id, OwnerID: owner, CatalogID: input.CatalogID, Name: name,
		Category: strings.TrimSpace(input.Category), Manufacturer: strings.TrimSpace(input.Manufacturer),
		Model: strings.TrimSpace(input.Model), SerialNumber: strings.TrimSpace(input.SerialNumber),
		ControllerType: strings.TrimSpace(input.ControllerType), LogicVoltage: input.LogicVoltage,
		Notes: strings.TrimSpace(input.Notes), CreatedAtMS: now, UpdatedAtMS: now,
	}
	stored, err := controller.product.CreateEquipment(ctx.Context(), equipment)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

func (controller *Controller) listEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	items, err := controller.product.ListEquipment(ctx.Context(), owner)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(items)
}

func (controller *Controller) getEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	item, err := controller.product.GetEquipment(ctx.Context(), owner, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if item == nil {
		return apiError(ctx, fiber.StatusNotFound, "EQUIPMENT_NOT_FOUND", "The requested equipment does not exist.")
	}
	return ctx.JSON(item)
}

func (controller *Controller) updateEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	var input struct {
		Name           *string  `json:"name"`
		Category       *string  `json:"category"`
		Manufacturer   *string  `json:"manufacturer"`
		Model          *string  `json:"model"`
		SerialNumber   *string  `json:"serial_number"`
		ControllerType *string  `json:"controller_type"`
		LogicVoltage   *float64 `json:"logic_voltage"`
		Notes          *string  `json:"notes"`
		CatalogID      *string  `json:"catalog_id"`
		CatalogIDSet   bool     `json:"-"`
	}
	raw := map[string]any{}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Equipment details must be valid JSON.")
	}
	// BodyParser cannot distinguish "catalog_id omitted" from "catalog_id
	// explicitly null" (both leave the field nil); a second, permissive
	// parse into a raw map recovers that distinction so PATCH can clear a
	// catalog reference without needing a separate endpoint.
	_ = ctx.BodyParser(&raw)
	_, catalogIDPresent := raw["catalog_id"]

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_EQUIPMENT", "Equipment name cannot be blank.")
		}
		input.Name = &trimmed
	}
	if catalogIDPresent && input.CatalogID != nil && strings.TrimSpace(*input.CatalogID) != "" {
		if !controller.validateCatalogID(*input.CatalogID) {
			return apiError(ctx, fiber.StatusUnprocessableEntity, "UNKNOWN_CATALOG_ID", "catalog_id does not match any known Component Catalog entry.")
		}
	}

	update := domain.EquipmentUpdate{
		Name: input.Name, Category: input.Category, Manufacturer: input.Manufacturer,
		Model: input.Model, SerialNumber: input.SerialNumber, ControllerType: input.ControllerType,
		LogicVoltage: input.LogicVoltage, Notes: input.Notes,
		CatalogIDSet: catalogIDPresent, CatalogID: input.CatalogID,
	}
	stored, err := controller.product.UpdateEquipment(ctx.Context(), owner, ctx.Params("id"), update)
	if err != nil {
		return internalError(ctx, err)
	}
	if stored == nil {
		return apiError(ctx, fiber.StatusNotFound, "EQUIPMENT_NOT_FOUND", "The requested equipment does not exist.")
	}
	return ctx.JSON(stored)
}

func (controller *Controller) deleteEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	err := controller.product.DeleteEquipment(ctx.Context(), owner, ctx.Params("id"))
	if err == domain.ErrNotFound {
		return apiError(ctx, fiber.StatusNotFound, "EQUIPMENT_NOT_FOUND", "The requested equipment does not exist.")
	}
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) listProjectEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	items, err := controller.product.ListProjectEquipment(ctx.Context(), owner, project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(items)
}

func (controller *Controller) attachProjectEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	var input struct {
		EquipmentID string `json:"equipment_id"`
		Role        string `json:"role"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Attachment details must be valid JSON.")
	}
	equipmentID := strings.TrimSpace(input.EquipmentID)
	if equipmentID == "" {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_ATTACHMENT", "equipment_id is required.")
	}
	equipment, err := controller.product.GetEquipment(ctx.Context(), owner, equipmentID)
	if err != nil {
		return internalError(ctx, err)
	}
	if equipment == nil {
		return apiError(ctx, fiber.StatusNotFound, "EQUIPMENT_NOT_FOUND", "The requested equipment does not exist.")
	}
	link, err := controller.product.AttachProjectEquipment(ctx.Context(), domain.ProjectEquipment{
		ProjectID: project.ID, EquipmentID: equipment.ID, OwnerID: owner, Role: strings.TrimSpace(input.Role),
	})
	if err == domain.ErrConflict {
		return apiError(ctx, fiber.StatusConflict, "ALREADY_ATTACHED", "This equipment is already attached to this project.")
	}
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(link)
}

func (controller *Controller) detachProjectEquipment(ctx *fiber.Ctx) error {
	if controller.product == nil {
		return productUnavailable(ctx)
	}
	owner, ok := requireOwner(ctx)
	if !ok {
		return nil
	}
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	err = controller.product.DetachProjectEquipment(ctx.Context(), owner, project.ID, ctx.Params("equipmentId"))
	if err == domain.ErrNotFound {
		return apiError(ctx, fiber.StatusNotFound, "ATTACHMENT_NOT_FOUND", "This equipment is not attached to this project.")
	}
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) listCatalog(ctx *fiber.Ctx) error {
	return ctx.JSON(controller.catalog)
}

func (controller *Controller) getCatalogEntry(ctx *fiber.Ctx) error {
	entry, ok := componentcatalog.Find(controller.catalog, ctx.Params("id"))
	if !ok {
		return apiError(ctx, fiber.StatusNotFound, "CATALOG_ENTRY_NOT_FOUND", "The requested catalog entry does not exist.")
	}
	return ctx.JSON(entry)
}
