package httpapi

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// apiAccessControl protects the HTTP and WebSocket API when a deployment
// explicitly configures a bearer token. With no token, the server must remain
// bound to loopback or an explicitly trusted, host-loopback-published network.
func apiAccessControl(token string) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if token == "" || ctx.Method() == fiber.MethodOptions {
			return ctx.Next()
		}
		header := ctx.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(header, "Bearer ") ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, "Bearer ")), []byte(token)) != 1 {
			ctx.Set("WWW-Authenticate", "Bearer")
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "UNAUTHORIZED"})
		}
		return ctx.Next()
	}
}
