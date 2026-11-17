package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/config"
	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
	"smartcoop/internal/middleware"
	"smartcoop/internal/security"
	"smartcoop/internal/service"
)

type AuthHandler struct {
	authSvc    *service.AuthService
	memberRepo domain.MemberRepository
	cfg        *config.Config
}

func NewAuthHandler(authSvc *service.AuthService, memberRepo domain.MemberRepository, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authSvc:    authSvc,
		memberRepo: memberRepo,
		cfg:        cfg,
	}
}

// RegisterAuthRoutes registers all authentication-related endpoints
func RegisterAuthRoutes(api fiber.Router, authSvc *service.AuthService, memberRepo domain.MemberRepository, cfg *config.Config) {
	h := NewAuthHandler(authSvc, memberRepo, cfg)

	g := api.Group("/auth")
	g.Post("/register", h.Register)
	g.Post("/login", h.Login)
	g.Post("/logout", h.Logout)
	g.Get("/me", middleware.RequireAuth(cfg.JWTSecret), h.Me)
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req domain.CreateMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	member, err := h.authSvc.Register(c.UserContext(), &req)
	if err != nil {
		return err
	}

	return httpx.WriteCreated(c, member)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req domain.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	member, err := h.authSvc.Login(c.UserContext(), &req)
	if err != nil {
		return err
	}

	ttl := time.Duration(h.cfg.JWTTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	claims := security.JWTClaims{
		NIM:   member.NIM,
		Email: member.Email,
		Name:  member.Name,
	}

	token, err := security.GenerateToken(h.cfg.JWTSecret, claims, ttl)
	if err != nil {
		return domain.NewInternalError("Gagal membuat token autentikasi")
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

	return httpx.WriteOK(c, fiber.Map{
		"member": member,
		"token":  token,
	})
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims := middleware.GetCurrentUser(c)
	if claims == nil {
		return domain.NewUnauthorizedError("Pengguna belum diautentikasi")
	}

	if h.memberRepo != nil {
		member, err := h.memberRepo.FindByNIM(c.UserContext(), claims.NIM)
		if err == nil && member != nil {
			return httpx.WriteOK(c, member)
		}
	}

	// Fallback ke data dari claims jika repo tidak tersedia
	return httpx.WriteOK(c, fiber.Map{
		"nim":   claims.NIM,
		"email": claims.Email,
		"nama":  claims.Name,
	})
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
