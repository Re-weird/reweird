package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
)

func (c *Controller) patchStatus(ctx *fiber.Ctx) error {
	if r, err := c.patchCapability("status"); err == nil {
		project, err := c.findProject(ctx, r.ProfileID)
		if err != nil {
			return internalError(ctx, err)
		}
		if project == nil || ownerID(ctx) == "" {
			return ctx.JSON(fiber.Map{"state": "PATCH LOCKED", "physical_enabled": false, "master_enabled": false, "detail": "Sign in as the physical project's owner to review PATCH capability."})
		}
		c.mu.RLock()
		master := c.patchEnabled[r.ProfileID] == r.DeviceID+":"+r.BootID+":"+r.ProbeMapHash
		c.mu.RUnlock()
		return ctx.JSON(fiber.Map{"state": "PATCH " + r.State, "physical_enabled": master, "master_enabled": master, "software_ready": true, "capability": r, "detail": "Verified protected output capability detected. Master enable and explicit approval are required for each bounded action."})
	}
	return ctx.JSON(fiber.Map{"state": "PATCH LOCKED", "physical_enabled": false, "master_enabled": false,
		"software_ready":              true,
		"physical_interface_verified": false, "physical_driver_available": false,
		"supported_physical_actions": []string{}, "detail": "PHYSICAL PATCH LOCKED — protected output hardware not detected"})
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
	if c.patchQualified(p) {
		r, err := c.patchCapability("status")
		if err != nil {
			return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
		}
		if r.State == "ACTIVE" || r.State == "ARMED" {
			return apiError(ctx, 409, "PATCH_BUSY", "An action is already armed or active")
		}
		if _, err = c.patchCapability("hello"); err != nil {
			return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
		}
		if _, err = c.physicalPatchDriver(p); err != nil {
			return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
		}
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
	if c.patch == nil {
		return serviceUnavailable(ctx, errors.New("PATCH unavailable"))
	}
	store := c.repository.(patchcontrol.Store)
	a, err := store.GetPatchAction(ctx.Params("action"))
	if err != nil {
		return internalError(ctx, err)
	}
	if a == nil || a.Owner != ownerID(ctx) || a.Parameters.ProfileID != project.ID {
		return fiber.ErrNotFound
	}
	if a.State == "LOCKED" {
		return apiError(ctx, 423, "PATCH_LOCKED", patchcontrol.ErrLocked.Error())
	}
	var input struct {
		Digest  string `json:"digest"`
		Confirm bool   `json:"confirm"`
	}
	if err := decodePatchBody(ctx, &input); err != nil {
		return err
	}
	if !input.Confirm {
		return apiError(ctx, 400, "APPROVAL_REQUIRED", "Explicit human approval required")
	}
	driver, err := c.physicalPatchDriver(a.Parameters)
	if err != nil {
		return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
	}
	if !c.patchMaster(a.Parameters) {
		return apiError(ctx, 423, "PATCH_LOCKED", "Master enable is OFF")
	}
	if _, err = c.patch.Approve(a.ID, input.Digest, ownerID(ctx)); err != nil {
		return apiError(ctx, 409, "INVALID_APPROVAL", err.Error())
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	result, err := c.patch.Run(runCtx, a.ID, driver)
	if err != nil {
		return ctx.Status(409).JSON(fiber.Map{"error": "PATCH_ABORTED", "detail": err.Error(), "action": result})
	}
	return ctx.JSON(result)
}

func decodePatchBody(ctx *fiber.Ctx, v any) error {
	if len(ctx.Body()) > 4096 {
		return fiber.ErrRequestEntityTooLarge
	}
	d := json.NewDecoder(bytes.NewReader(ctx.Body()))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return apiError(ctx, 400, "INVALID_PATCH", err.Error())
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return apiError(ctx, 400, "INVALID_PATCH", "Exactly one JSON object required")
	}
	return nil
}
func (c *Controller) patchMasterEnable(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return fiber.ErrUnauthorized
	}
	p, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if p == nil {
		return fiber.ErrNotFound
	}
	var input struct {
		Enabled bool `json:"enabled"`
		Confirm bool `json:"confirm"`
	}
	if err := decodePatchBody(ctx, &input); err != nil {
		return err
	}
	if !input.Enabled {
		c.mu.Lock()
		delete(c.patchEnabled, p.ID)
		c.mu.Unlock()
		if t, ok := c.source.(patchcontrol.Transport); ok {
			cx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, _ = t.PatchExchange(cx, patchcontrol.Command{Op: "disable"})
		}
		return ctx.JSON(fiber.Map{"master_enabled": false})
	}
	if !input.Confirm {
		return apiError(ctx, 400, "APPROVAL_REQUIRED", "Explicit master enable confirmation required")
	}
	r, err := c.patchCapability("status")
	if err != nil || r.ProfileID != p.ID {
		return apiError(ctx, 423, "PATCH_LOCKED", patchcontrol.ErrLocked.Error())
	}
	c.mu.Lock()
	c.patchEnabled[p.ID] = r.DeviceID + ":" + r.BootID + ":" + r.ProbeMapHash
	c.mu.Unlock()
	return ctx.JSON(fiber.Map{"master_enabled": true})
}
func (c *Controller) preparePatch(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return fiber.ErrUnauthorized
	}
	project, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return fiber.ErrNotFound
	}
	var input struct {
		Level      string `json:"level"`
		DurationMS int    `json:"duration_ms"`
	}
	if err := decodePatchBody(ctx, &input); err != nil {
		return err
	}
	r, err := c.patchCapability("status")
	if err != nil || r.ProfileID != project.ID {
		return apiError(ctx, 423, "PATCH_LOCKED", patchcontrol.ErrLocked.Error())
	}
	if r.State == "ARMED" || r.State == "ACTIVE" {
		return apiError(ctx, 409, "PATCH_BUSY", "Action already pending")
	}
	r, err = c.patchCapability("hello")
	if err != nil {
		return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
	}
	p := patchcontrol.Parameters{ProfileID: r.ProfileID, ProfileRevision: r.ProfileRevision, ProbeMapHash: r.ProbeMapHash, DeviceID: r.DeviceID, BootID: r.BootID, TargetNode: r.TargetNode, PatchPin: r.Pin, Mode: "PULSE", LogicLevel: input.Level, MaxVoltage: 3.3, DurationMS: input.DurationMS, ExpiresAtMS: time.Now().UnixMilli() + 30000, Source: "REAL_SERIAL"}
	if _, err = c.physicalPatchDriver(p); err != nil {
		return apiError(ctx, 423, "PATCH_LOCKED", err.Error())
	}
	if c.patch == nil {
		return serviceUnavailable(ctx, errors.New("PATCH storage unavailable"))
	}
	a, err := c.patch.Propose(p, ownerID(ctx))
	if err != nil {
		return apiError(ctx, 400, "INVALID_PATCH", err.Error())
	}
	return ctx.Status(201).JSON(a)
}
func (c *Controller) cancelPatch(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return fiber.ErrUnauthorized
	}
	project, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return fiber.ErrNotFound
	}
	s, ok := c.repository.(patchcontrol.Store)
	if !ok || c.patch == nil {
		return serviceUnavailable(ctx, errors.New("PATCH storage unavailable"))
	}
	a, err := s.GetPatchAction(ctx.Params("action"))
	if err != nil {
		return internalError(ctx, err)
	}
	if a == nil || a.Owner != ownerID(ctx) || a.Parameters.ProfileID != project.ID {
		return fiber.ErrNotFound
	}
	c.mu.Lock()
	delete(c.patchEnabled, project.ID)
	c.mu.Unlock()
	if t, ok := c.source.(patchcontrol.Transport); ok {
		cx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, _ = t.PatchExchange(cx, patchcontrol.Command{Op: "disable", ActionID: a.ID})
	}
	result, err := c.patch.Cancel(a.ID, ownerID(ctx))
	if err != nil {
		return apiError(ctx, 409, "PATCH_CANCEL_FAILED", err.Error())
	}
	return ctx.JSON(result)
}
