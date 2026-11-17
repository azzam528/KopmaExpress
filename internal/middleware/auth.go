package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/security"
)

const (
	AuthCookieName = "auth_token"
	ContextUserKey = "user"
)

// RequireAuth verifies presence and validity of JWT from HttpOnly cookie or Authorization header
func RequireAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Cookies(AuthCookieName)

		if tokenStr == "" {
			authHeader := c.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenStr == "" {
			return domain.NewUnauthorizedError("Silakan masuk terlebih dahulu untuk mengakses layanan ini")
		}

		claims, err := security.ValidateToken(jwtSecret, tokenStr)
		if err != nil {
			return domain.NewUnauthorizedError("Sesi Anda tidak valid atau telah berakhir, silakan masuk kembali")
		}

		c.Locals(ContextUserKey, claims)
		return c.Next()
	}
}

// GetCurrentUser retrieves the authenticated user claims from the Fiber context
func GetCurrentUser(c *fiber.Ctx) *security.JWTClaims {
	val := c.Locals(ContextUserKey)
	if val == nil {
		return nil
	}
	if claims, ok := val.(*security.JWTClaims); ok {
		return claims
	}
	return nil
}
