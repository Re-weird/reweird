package httpapi

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	websocket "github.com/gofiber/websocket/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// telemetryPollInterval bounds how often the WebSocket push loop re-checks
// a telemetry source that cannot signal new frames directly (e.g. the
// demo simulator, which implements domain.TelemetrySource but not
// domain.FreshTelemetrySource).
const telemetryPollInterval = 500 * time.Millisecond

// telemetryWebSocketUpgrade only allows the WebSocket handshake to proceed;
// it never accepts raw telemetry input, matching the read-only push
// contract described in docs/backend-checklist.md.
func telemetryWebSocketUpgrade(ctx *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(ctx) {
		return ctx.Next()
	}
	return fiber.ErrUpgradeRequired
}

// telemetryWebSocket streams the current diagnostic session as JSON each
// time a new telemetry frame arrives, so the frontend no longer has to
// poll /api/v1/measurements or /api/v1/session. It only pushes normalized
// sessions the diagnostic engine already produced -- it never streams raw
// telemetry directly and never accepts client input beyond ping/close.
func (controller *Controller) telemetryWebSocket() fiber.Handler {
	return websocket.New(func(connection *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Detect client disconnects promptly so the push loop below does
		// not block forever on a dead connection.
		go func() {
			defer cancel()
			for {
				if _, _, err := connection.ReadMessage(); err != nil {
					return
				}
			}
		}()

		session, err := controller.analyze(ctx)
		if err != nil {
			_ = connection.WriteJSON(fiber.Map{"error": "ANALYSIS_UNAVAILABLE", "detail": err.Error()})
			return
		}
		if err := connection.WriteJSON(session); err != nil {
			return
		}

		sequence := session.RawTelemetry.Sequence
		waiter, hasWaiter := controller.source.(domain.FreshTelemetrySource)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if hasWaiter {
				waitContext, waitCancel := context.WithTimeout(ctx, 30*time.Second)
				_, err := waiter.WaitNext(waitContext, sequence)
				waitCancel()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					// Timeout waiting for a new frame; loop and check
					// again rather than treating it as a fatal error.
					continue
				}
			} else {
				// The active source (e.g. the demo simulator) cannot
				// signal new frames directly, so poll it at a bounded
				// interval instead of blocking forever.
				select {
				case <-ctx.Done():
					return
				case <-time.After(telemetryPollInterval):
				}
			}
			session, err := controller.analyze(ctx)
			if err != nil {
				log.Printf("telemetry websocket: analyze: %v", err)
				continue
			}
			if session.RawTelemetry.Sequence == sequence {
				continue
			}
			sequence = session.RawTelemetry.Sequence
			if err := connection.WriteJSON(session); err != nil {
				return
			}
		}
	})
}
