package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
	"github.com/re-weird/reweird/apps/api/internal/transport/serialsource"
)

func bridgeHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func bridgeSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func profileDigest(p domain.ProjectProfile) string {
	b, _ := json.Marshal(p)
	return bridgeHash(string(b))
}

// No PatchExchange / PatchLive implementation: this transport is input-only.
type bridgeSource struct {
	store    domain.BridgeRepository
	id       string
	profiles domain.Repository
}

func (s *bridgeSource) Name() string { return "serial" }
func (s *bridgeSource) Latest(context.Context) (domain.TelemetryEnvelope, error) {
	b, err := s.store.GetBridge(s.id)
	if err != nil {
		return domain.TelemetryEnvelope{}, err
	}
	if b == nil || b.ExpiresAtMS <= time.Now().UnixMilli() {
		return domain.TelemetryEnvelope{}, errors.New("USB bridge pairing expired or revoked; pair again")
	}
	if b.LastError != "" {
		return domain.TelemetryEnvelope{}, errors.New(b.LastError)
	}
	if b.Raw == nil || time.Now().UnixMilli()-b.ReceivedAtMS > 5000 {
		return domain.TelemetryEnvelope{}, errors.New("Device offline: no fresh USB bridge capture within 5 seconds")
	}
	p, err := s.profiles.GetProfile(s.id)
	if err != nil {
		return domain.TelemetryEnvelope{}, err
	}
	if p == nil || profileDigest(*p) != b.ProfileHash {
		return domain.TelemetryEnvelope{}, errors.New("Profile changed: review probe mapping and pair the USB bridge again")
	}
	e := *b.Raw
	e.ProfileID = b.ProjectID // Explicit owner-confirmed wire-profile alias, original retained in binding.
	e.CapturedAtMS = b.SentAtMS
	return e, nil
}
func (s *bridgeSource) WaitNext(ctx context.Context, after uint64) (domain.TelemetryEnvelope, error) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		if e, err := s.Latest(ctx); err == nil && e.Sequence > after {
			return e, nil
		}
		select {
		case <-ctx.Done():
			return domain.TelemetryEnvelope{}, ctx.Err()
		case <-t.C:
		}
	}
}

func (c *Controller) bridgeController(id string) (*Controller, error) {
	store, ok := c.repository.(domain.BridgeRepository)
	if !ok {
		return nil, nil
	}
	b, err := store.GetBridge(id)
	if err != nil || b == nil {
		return nil, err
	}
	c.bridgeMu.Lock()
	defer c.bridgeMu.Unlock()
	if child := c.bridgeControllers[id]; child != nil {
		return child, nil
	}
	child := &Controller{repository: c.repository, engine: c.engine, source: &bridgeSource{store: store, id: id, profiles: c.repository}, profileID: id, stage: domain.StageDiagnose, patchEnabled: map[string]string{}, understanding: c.understanding, uploadRoot: c.uploadRoot}
	c.bridgeControllers[id] = child
	return child, nil
}

// Only the verified project owner can route existing analysis/test handlers to
// a cloud source. No global active source is replaced, including the simulator.
func (c *Controller) bridgeScoped(fn func(*Controller, *fiber.Ctx) error) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		id := ctx.Query("project_id")
		if strings.HasPrefix(ctx.Path(), "/api/v1/tests/") && ctx.Params("id") != "" {
			if repo, ok := c.repository.(domain.TestWorkflowRepository); ok {
				w, err := repo.GetTestWorkflow(ctx.Params("id"))
				if err != nil {
					return internalError(ctx, err)
				}
				if w != nil {
					if id != "" && id != w.ProjectID {
						return apiError(ctx, 404, "TEST_NOT_FOUND", "Test not found for this project.")
					}
					if store, ok := c.repository.(domain.BridgeRepository); ok {
						b, err := store.GetBridge(w.ProjectID)
						if err != nil {
							return internalError(ctx, err)
						}
						if b != nil {
							id = w.ProjectID
						}
					}
				}
			}
		}
		if id == "" && strings.HasPrefix(ctx.Path(), "/api/v1/profiles/") {
			id = ctx.Params("id")
		}
		if id == "" || id == "demo" || id == "ultrasonic-demo" {
			return fn(c, ctx)
		}
		p, err := c.findProject(ctx, id)
		if err != nil {
			return internalError(ctx, err)
		}
		if p == nil {
			return apiError(ctx, 404, "PROJECT_NOT_FOUND", "Project not found.")
		}
		if strings.HasPrefix(ctx.Path(), "/api/v1/tests/") && ctx.Params("id") != "" {
			if repo, ok := c.repository.(domain.TestWorkflowRepository); ok {
				w, err := repo.GetTestWorkflow(ctx.Params("id"))
				if err != nil {
					return internalError(ctx, err)
				}
				if w == nil || w.ProjectID != id {
					return apiError(ctx, 404, "TEST_NOT_FOUND", "Test not found for this project.")
				}
			}
		}
		child, err := c.bridgeController(id)
		if err != nil {
			return internalError(ctx, err)
		}
		if child != nil {
			return fn(child, ctx)
		}
		return fn(c, ctx)
	}
}

func (c *Controller) pairBridge(ctx *fiber.Ctx) error {
	if ownerID(ctx) == "" {
		return apiError(ctx, 401, "SIGN_IN_REQUIRED", "Sign in to pair physical hardware.")
	}
	p, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if p == nil {
		return apiError(ctx, 404, "PROJECT_NOT_FOUND", "Project not found.")
	}
	var input struct {
		DeviceID       string `json:"device_id"`
		WireProfileID  string `json:"wire_profile_id"`
		ConfirmMapping bool   `json:"confirm_mapping"`
	}
	if ctx.BodyParser(&input) != nil || !input.ConfirmMapping || len(input.DeviceID) < 1 || len(input.DeviceID) > 128 || len(input.WireProfileID) < 1 || len(input.WireProfileID) > 128 {
		return apiError(ctx, 422, "PAIRING_CONFIRMATION_REQUIRED", "Provide device ID, firmware profile ID and confirm the physical mapping.")
	}
	profile, err := c.repository.GetProfile(p.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil || !profile.Confirmed || p.ProbePlan == nil || !p.ProbePlan.Connected {
		return apiError(ctx, 409, "PROFILE_NOT_READY", "Confirm the physical profile and probe placement first.")
	}
	store, ok := c.repository.(domain.BridgeRepository)
	if !ok {
		return apiError(ctx, 503, "BRIDGE_UNAVAILABLE", "Bridge storage unavailable.")
	}
	token, err := bridgeSecret()
	if err != nil {
		return internalError(ctx, err)
	}
	share, err := bridgeSecret()
	if err != nil {
		return internalError(ctx, err)
	}
	b := domain.BridgeBinding{ProjectID: p.ID, OwnerID: p.OwnerID, DeviceID: input.DeviceID, WireProfileID: input.WireProfileID, ProfileHash: profileDigest(*profile), TokenHash: bridgeHash(token), ShareHash: bridgeHash(share), ExpiresAtMS: time.Now().Add(12 * time.Hour).UnixMilli()}
	c.bridgeIngestMu.Lock()
	defer c.bridgeIngestMu.Unlock()
	active, err := store.ActiveBridgeForDevice(input.DeviceID)
	if err != nil {
		return internalError(ctx, err)
	}
	if active != nil && active.ProjectID != p.ID {
		return apiError(ctx, 409, "DEVICE_ALREADY_PAIRED", "Revoke this device's existing project pairing before pairing it elsewhere.")
	}
	if err = store.SaveBridge(b); err != nil {
		return internalError(ctx, err)
	}
	c.bridgeMu.Lock()
	delete(c.bridgeControllers, p.ID)
	c.bridgeMu.Unlock()
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{"token": token, "share_token": share, "expires_at_ms": b.ExpiresAtMS, "project_id": p.ID})
}

func (c *Controller) revokeBridge(ctx *fiber.Ctx) error {
	p, err := c.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if p == nil || ownerID(ctx) == "" {
		return apiError(ctx, 404, "PROJECT_NOT_FOUND", "Project not found.")
	}
	store, ok := c.repository.(domain.BridgeRepository)
	if !ok {
		return apiError(ctx, 503, "BRIDGE_UNAVAILABLE", "Bridge storage unavailable.")
	}
	c.bridgeIngestMu.Lock()
	defer c.bridgeIngestMu.Unlock()
	b, err := store.GetBridge(p.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if b != nil {
		b.ExpiresAtMS = 0
		b.TokenHash = ""
		b.ShareHash = ""
		if err = store.SaveBridge(*b); err != nil {
			return internalError(ctx, err)
		}
	}
	return ctx.SendStatus(204)
}

// Tokens use dedicated headers, never query strings (access logs) or owner JWTs.
func (c *Controller) authorizedBridge(ctx *fiber.Ctx, share bool) (*domain.BridgeBinding, error) {
	store, ok := c.repository.(domain.BridgeRepository)
	if !ok {
		return nil, errors.New("bridge unavailable")
	}
	b, err := store.GetBridge(ctx.Params("id"))
	if err != nil || b == nil {
		return nil, errors.New("invalid or expired bridge credential")
	}
	project, err := c.repository.GetProject(b.ProjectID)
	if err != nil || project == nil || project.OwnerID != b.OwnerID {
		return nil, errors.New("pairing owner no longer matches project")
	}
	header, hash := "X-ReWeird-Bridge", b.TokenHash
	if share {
		header, hash = "X-ReWeird-Share", b.ShareHash
	}
	token := ctx.Get(header)
	if len(token) != 64 || subtle.ConstantTimeCompare([]byte(bridgeHash(token)), []byte(hash)) != 1 || b.ExpiresAtMS <= time.Now().UnixMilli() {
		return nil, errors.New("invalid or expired bridge credential")
	}
	return b, nil
}

func (c *Controller) ingestBridge(ctx *fiber.Ctx) error {
	c.bridgeIngestMu.Lock()
	defer c.bridgeIngestMu.Unlock()
	b, err := c.authorizedBridge(ctx, false)
	if err != nil {
		return apiError(ctx, 401, "BRIDGE_UNAUTHORIZED", err.Error())
	}
	if len(ctx.Body()) > 270000 {
		return apiError(ctx, 413, "FRAME_TOO_LARGE", "Frame exceeds bridge limit.")
	}
	reject := func(status int, code, detail string) error {
		b.LastError = detail
		if err := c.repository.(domain.BridgeRepository).SaveBridge(*b); err != nil {
			return internalError(ctx, err)
		}
		return apiError(ctx, status, code, detail)
	}
	var input struct {
		SentAtMS int64           `json:"sent_at_ms"`
		Frame    json.RawMessage `json:"frame"`
	}
	if ctx.BodyParser(&input) != nil {
		return reject(400, "INVALID_FRAME", "Invalid bridge packet.")
	}
	now := time.Now().UnixMilli()
	if input.SentAtMS < now-5000 || input.SentAtMS > now+2000 {
		return reject(422, "STALE_FRAME", "Capture is stale or laptop clock is incorrect; do not buffer telemetry.")
	}
	e, err := serialsource.DecodeLine(input.Frame)
	if err != nil {
		return reject(422, "INVALID_FRAME", err.Error())
	}
	if e.DeviceID != b.DeviceID || e.ProfileID != b.WireProfileID {
		return reject(409, "PAIRING_MISMATCH", fmt.Sprintf("Expected device %s / firmware profile %s; received %s / %s", b.DeviceID, b.WireProfileID, e.DeviceID, e.ProfileID))
	}
	if b.Raw != nil && (e.Sequence <= b.Raw.Sequence || e.UptimeMS < b.Raw.UptimeMS || input.SentAtMS <= b.SentAtMS) {
		return apiError(ctx, 409, "REPLAY_OR_REBOOT", "Sequence did not advance. After device reboot, explicitly pair again.")
	}
	if b.ReceivedAtMS > 0 && now-b.ReceivedAtMS < 200 {
		return apiError(ctx, 429, "BRIDGE_RATE_LIMIT", "Maximum five frames per second.")
	}
	p, err := c.repository.GetProfile(b.ProjectID)
	if err != nil {
		return internalError(ctx, err)
	}
	if p == nil || !p.Confirmed || profileDigest(*p) != b.ProfileHash {
		return reject(409, "PROFILE_CHANGED", "Profile changed; confirm and pair again.")
	}
	normalized := e
	normalized.ProfileID = p.ID
	if err = telemetry.ValidateForProfile(normalized, *p); err != nil {
		return reject(422, "PROFILE_VALIDATION_FAILED", err.Error())
	}
	b.Raw = &e
	b.LastError = ""
	b.ReceivedAtMS = now
	b.SentAtMS = input.SentAtMS
	if err = c.repository.(domain.BridgeRepository).SaveBridge(*b); err != nil {
		return internalError(ctx, err)
	}
	// Persist EVERY accepted window, not just the windows a browser polls.
	// Consecutive-window calibration must not depend on a judge keeping a tab open.
	child, err := c.bridgeController(b.ProjectID)
	if err != nil {
		return internalError(ctx, err)
	}
	// Raw ingestion must not wait on remote PROBE/LLM calls. Use the existing
	// deterministic engine here; owner-requested diagnosis still uses c.engine.
	currentProfile, err := child.profileWithKnownGood(*p, normalized)
	if err != nil {
		return internalError(ctx, err)
	}
	normalized.CapturedAtMS = input.SentAtMS
	session, err := diagnostics.NewEngine(signalanalysis.New()).AnalyzeEnvelope(ctx.Context(), currentProfile, domain.StageDiagnose, "serial", normalized, nil)
	if err != nil {
		return serviceUnavailable(ctx, err)
	}
	measurements, ok := c.repository.(domain.MeasurementRepository)
	if !ok {
		return apiError(ctx, 503, "MEASUREMENT_STORAGE_UNAVAILABLE", "Measurement storage unavailable.")
	}
	window, err := measurements.SaveMeasurement(domain.MeasurementWindow{ProfileID: p.ID, Source: "serial", DeviceID: normalized.DeviceID, Sequence: normalized.Sequence, CapturedAtMS: normalized.CapturedAtMS, Raw: normalized, Analysis: session.Analysis})
	if err != nil {
		return internalError(ctx, err)
	}
	session.MeasurementID = window.ID
	session.RawTelemetry = normalized
	child.bridgeSessionMu.Lock()
	child.bridgeSession = &session
	child.bridgeSessionMu.Unlock()
	return ctx.JSON(fiber.Map{"accepted": true, "sequence": e.Sequence, "source": "REAL_SERIAL", "transport": "usb_bridge"})
}

func (c *Controller) judgeBridge(ctx *fiber.Ctx) error {
	b, err := c.authorizedBridge(ctx, true)
	if err != nil {
		return apiError(ctx, 401, "SHARE_UNAUTHORIZED", err.Error())
	}
	child, err := c.bridgeController(b.ProjectID)
	if err != nil {
		return internalError(ctx, err)
	}
	ctx.Set("Cache-Control", "no-store")
	ctx.Set("Referrer-Policy", "no-referrer")
	result := fiber.Map{"connected": false, "source": "REAL_SERIAL", "transport": "usb_bridge", "patch": "LOCKED", "last_capture_at_ms": b.SentAtMS, "device_id": b.DeviceID, "profile_id": b.ProjectID}
	if _, err = child.source.Latest(ctx.Context()); err != nil {
		result["error"] = err.Error()
		return ctx.JSON(result)
	}
	// Only the measurement projection is public. Never expose source code,
	// photos, owner identity, action endpoints, or general repository access.
	child.bridgeSessionMu.RLock()
	session := child.bridgeSession
	child.bridgeSessionMu.RUnlock()
	if b.Raw == nil || session == nil || session.RawTelemetry.Sequence != b.Raw.Sequence || session.RawTelemetry.CapturedAtMS != b.SentAtMS {
		result["error"] = "Waiting for the next validated cloud capture."
		return ctx.JSON(result)
	}
	result["connected"] = true
	result["probes"] = session.Probes
	result["measurement_id"] = session.MeasurementID
	result["sequence"] = session.RawTelemetry.Sequence
	return ctx.JSON(result)
}
