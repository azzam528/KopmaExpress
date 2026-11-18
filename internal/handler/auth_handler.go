package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"smartcoop/internal/config"
	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
	"smartcoop/internal/middleware"
	"smartcoop/internal/service"
)

type AuthHandler struct {
	authSvc *service.AuthService
	cfg     *config.Config
}

func NewAuthHandler(authSvc *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc,
		cfg:     cfg,
	}
}

// RegisterAuthRoutes registers all authentication-related API endpoints under /auth
func RegisterAuthRoutes(api fiber.Router, authSvc *service.AuthService, cfg *config.Config) {
	h := NewAuthHandler(authSvc, cfg)

	g := api.Group("/auth")
	g.Post("/login", h.Login)
	g.Post("/logout", h.Logout)
	g.Get("/me", middleware.RequireAuth(cfg.JWTSecret), h.Me)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req domain.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	user, member, token, err := h.authSvc.Login(c.UserContext(), &req)
	if err != nil {
		return err
	}

	ttl := time.Duration(h.cfg.JWTTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	isSecure := c.Protocol() == "https" || h.cfg.AppEnv == "production"

	c.Cookie(&fiber.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	redirectURL := "/"
	if user.Role == domain.RoleMember {
		redirectURL = "/belanja"
	}

	res := fiber.Map{
		"user": fiber.Map{
			"id":     user.ID.Hex(),
			"nim":    user.NIM,
			"name":   user.Name,
			"role":   user.Role,
			"status": user.Status,
		},
		"token":        token,
		"redirect_url": redirectURL,
	}

	if member != nil {
		res["member"] = fiber.Map{
			"id":       member.ID.Hex(),
			"nim":      member.NIM,
			"nama":     member.Name,
			"unit":     member.Unit,
			"telepon":  member.Phone,
			"is_active": member.IsActive,
		}
	}

	return httpx.WriteOK(c, res)
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims := middleware.GetCurrentUser(c)
	if claims == nil {
		return domain.NewUnauthorizedError("Pengguna belum diautentikasi")
	}

	var uid primitive.ObjectID
	if claims.UserID != "" {
		uid, _ = primitive.ObjectIDFromHex(claims.UserID)
	}

	profile, err := h.authSvc.GetProfile(c.UserContext(), uid, claims.NIM)
	if err != nil || profile == nil {
		// Fallback safely to token claims if user lookup failed
		return httpx.WriteOK(c, fiber.Map{
			"user_id": claims.UserID,
			"nim":     claims.NIM,
			"name":    claims.Name,
			"role":    claims.Role,
			"status":  domain.UserStatusActive,
		})
	}

	return httpx.WriteOK(c, profile)
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	isSecure := c.Protocol() == "https" || h.cfg.AppEnv == "production"

	c.Cookie(&fiber.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-100 * time.Hour),
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	return httpx.WriteOK(c, fiber.Map{
		"message": "Berhasil keluar dari akun",
	})
}
