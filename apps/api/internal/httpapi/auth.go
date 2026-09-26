package httpapi

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const ownerIDLocalsKey = "owner_id"

// ownerContext verifies an optional bearer token minted by apps/web's
// /api/auth/token route and records the verified caller identity for
// downstream handlers via ctx.Locals("owner_id"). It never trusts anything
// the client claims about its own identity - only a signature verified
// against the shared AUTH_TOKEN_SECRET sets owner_id.
//
// Behavior:
//   - No Authorization header at all -> anonymous/demo caller. owner_id
//     stays empty and the request proceeds; Demo Mode must keep working
//     without an account.
//   - AUTH_TOKEN_SECRET isn't configured on this backend -> same as above.
//     There is no verifier to check a token against yet, so every caller is
//     treated as anonymous rather than failing the whole API closed.
//   - Header present and the token verifies -> owner_id is set to the
//     verified subject claim (the caller's Google account id).
//   - Header present but invalid, malformed, expired, or unverifiable ->
//     401 Unauthorized immediately. A bad token is never silently
//     downgraded into an anonymous request.
func ownerContext(authConfigured bool) fiber.Handler {
	secret := []byte(os.Getenv("AUTH_TOKEN_SECRET"))
	return func(ctx *fiber.Ctx) error {
		header := ctx.Get("Authorization")
		if header == "" || !authConfigured {
			return ctx.Next()
		}
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			return apiError(ctx, fiber.StatusUnauthorized, "INVALID_AUTHORIZATION", "The Authorization header must use the Bearer scheme.")
		}
		claims := &jwt.RegisteredClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return secret, nil
		})
		if err != nil || strings.TrimSpace(claims.Subject) == "" {
			return apiError(ctx, fiber.StatusUnauthorized, "INVALID_TOKEN", "The provided session token could not be verified.")
		}
		ctx.Locals(ownerIDLocalsKey, claims.Subject)
		return ctx.Next()
	}
}

// ownerID returns the verified caller identity for this request, or "" for
// an anonymous/Demo Mode caller. It only ever reflects a value ownerContext
// set from a verified token - a request can never set this itself.
func ownerID(ctx *fiber.Ctx) string {
	value, _ := ctx.Locals(ownerIDLocalsKey).(string)
	return value
}
