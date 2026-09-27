package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
)

func (c *Controller) patchStatus(ctx *fiber.Ctx) error {
	return ctx.JSON(fiber.Map{"state": "PATCH LOCKED", "physical_enabled": false, "master_enabled": false,
		"physical_interface_verified": patchcontrol.PhysicalInterfaceVerified, "physical_driver_available": false,
		"supported_physical_actions": []string{}, "detail": patchcontrol.ErrLocked.Error()})
}

func (c *Controller) patchActions(ctx *fiber.Ctx) error {
	project, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if project == nil {
		return fiber.ErrNotFound
	}
	store, ok := c.repository.(patchcontrol.Store)
	if !ok || c.patch == nil {
		return serviceUnavailable(ctx, errors.New("PATCH audit storage unavailable"))
	}
	actions, err := store.ListPatchActions(project.ID)
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	return ctx.JSON(actions)
}

// Proposals never execute hardware and cannot carry client-supplied approval,
// execution status or actor identity. The existing /patch execution route stays
// HTTP 423. AI can propose parameters but cannot call through to a serial port.
func (c *Controller) proposePatch(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return fiber.ErrUnauthorized
	}
	project, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if project == nil {
		return fiber.ErrNotFound
	}
	if c.patch == nil {
		return serviceUnavailable(ctx, errors.New("PATCH audit storage unavailable"))
	}
	if len(ctx.Body()) > 4096 {
		return fiber.ErrRequestEntityTooLarge
	}
	var p patchcontrol.Parameters
	decoder := json.NewDecoder(bytes.NewReader(ctx.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return apiError(ctx, 400, "INVALID_PATCH", err.Error())
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return apiError(ctx, 400, "INVALID_PATCH", "exactly one JSON object required")
	}
	if p.ProfileID != project.ID || p.Source != "REAL_SERIAL" {
		return apiError(ctx, 400, "INVALID_PATCH", "physical proposal must match project and REAL_SERIAL provenance; practice cannot request hardware")
	}
	profile, err := c.repository.GetProfile(project.ID)
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if profile == nil || !profile.Confirmed || profile.Version != p.ProfileRevision {
		return apiError(ctx, 400, "INVALID_PATCH", "confirmed matching profile revision required")
	}
	a, err := c.patch.Propose(p, ownerID(ctx))
	if errors.Is(err, patchcontrol.ErrLocked) {
		return ctx.Status(423).JSON(fiber.Map{"error": "PATCH_LOCKED", "detail": err.Error(), "action": a})
	}
	if err != nil {
		return apiError(ctx, 400, "INVALID_PATCH", err.Error())
	}
	return ctx.Status(201).JSON(a)
}

func (c *Controller) approvePatch(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return fiber.ErrUnauthorized
	}
	project, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	if project == nil {
		return fiber.ErrNotFound
	}
	// Deliberately no activation API, environment bypass or physical driver.
	// Approval UI must remain disabled until the hardware release gate is met.
	return ctx.Status(423).JSON(fiber.Map{"error": "PATCH_LOCKED", "detail": patchcontrol.ErrLocked.Error()})
}
