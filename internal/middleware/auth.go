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

// RequireAuth verifies presence and validity of JWT from HttpOnly cookie or Authorization header for API routes
func RequireAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractToken(c)
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

// RequireRole checks whether the authenticated user possesses one of the required roles
func RequireRole(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetCurrentUser(c)
		if claims == nil {
			return domain.NewUnauthorizedError("Silakan masuk terlebih dahulu")
		}

		userRole := strings.ToLower(strings.TrimSpace(claims.Role))
		for _, r := range allowedRoles {
			if userRole == strings.ToLower(strings.TrimSpace(r)) {
				return c.Next()
			}
		}

		return domain.NewForbiddenError("Akses ditolak: Anda tidak memiliki izin untuk mengakses resource ini")
	}
}

// WebRequireAuth protects HTML web pages, redirecting unauthenticated users to /login
// and redirecting members attempting to access admin pages to /belanja.
func WebRequireAuth(jwtSecret string, allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			return c.Redirect("/login")
		}

		claims, err := security.ValidateToken(jwtSecret, tokenStr)
		if err != nil {
			return c.Redirect("/login")
		}

		c.Locals(ContextUserKey, claims)

		if len(allowedRoles) > 0 {
			userRole := strings.ToLower(strings.TrimSpace(claims.Role))
			isAllowed := false
			for _, r := range allowedRoles {
				if userRole == strings.ToLower(strings.TrimSpace(r)) {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				if userRole == domain.RoleMember {
					return c.Redirect("/belanja")
				}
				return c.Redirect("/login")
			}
		}

		return c.Next()
	}
}

// WebRequireGuest ensures that logged-in users are redirected away from public login pages
// according to their role (admin -> /, member -> /belanja).
func WebRequireGuest(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr != "" {
			if claims, err := security.ValidateToken(jwtSecret, tokenStr); err == nil && claims != nil {
				c.Locals(ContextUserKey, claims)
				if strings.ToLower(claims.Role) == domain.RoleMember {
					return c.Redirect("/belanja")
				}
				return c.Redirect("/")
			}
		}
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

func extractToken(c *fiber.Ctx) string {
	tokenStr := c.Cookies(AuthCookieName)
	if tokenStr == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	return strings.TrimSpace(tokenStr)
}
