package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"smartcoop/internal/config"
	"smartcoop/internal/domain"
	"smartcoop/internal/middleware"
	"smartcoop/internal/security"
	"smartcoop/internal/service"
)

// In-memory repositories for handler testing
type testUserRepo struct {
	users map[string]*domain.User
}

func newTestUserRepo() *testUserRepo {
	return &testUserRepo{users: make(map[string]*domain.User)}
}

func (r *testUserRepo) FindByNIM(_ context.Context, nim string) (*domain.User, error) {
	if u, ok := r.users[strings.TrimSpace(nim)]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, domain.NewNotFoundError("user")
}

func (r *testUserRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("user")
}

func (r *testUserRepo) FindByMemberID(_ context.Context, memberID primitive.ObjectID) (*domain.User, error) {
	for _, u := range r.users {
		if u.MemberID != nil && *u.MemberID == memberID {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("user")
}

func (r *testUserRepo) Create(_ context.Context, u *domain.User) error {
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	cp := *u
	r.users[u.NIM] = &cp
	return nil
}

func (r *testUserRepo) Update(_ context.Context, id primitive.ObjectID, u *domain.User) error {
	for k, existing := range r.users {
		if existing.ID == id {
			cp := *u
			cp.ID = id
			r.users[k] = &cp
			return nil
		}
	}
	return domain.NewNotFoundError("user")
}

func (r *testUserRepo) Count(_ context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

type testMemberRepo struct {
	members map[string]*domain.Member
}

func newTestMemberRepo() *testMemberRepo {
	return &testMemberRepo{members: make(map[string]*domain.Member)}
}

func (r *testMemberRepo) FindAll(_ context.Context, _ domain.MemberFilter) ([]domain.Member, int64, error) {
	res := make([]domain.Member, 0, len(r.members))
	for _, m := range r.members {
		res = append(res, *m)
	}
	return res, int64(len(res)), nil
}

func (r *testMemberRepo) FindByNIM(_ context.Context, nim string) (*domain.Member, error) {
	if m, ok := r.members[strings.TrimSpace(nim)]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, domain.NewNotFoundError("member")
}

func (r *testMemberRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Member, error) {
	for _, m := range r.members {
		if m.ID == id {
			cp := *m
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("member")
}

func (r *testMemberRepo) FindByEmail(_ context.Context, email string) (*domain.Member, error) {
	for _, m := range r.members {
		if strings.EqualFold(m.Email, strings.TrimSpace(email)) {
			cp := *m
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("member")
}

func (r *testMemberRepo) Create(_ context.Context, m *domain.Member) error {
	if m.ID.IsZero() {
		m.ID = primitive.NewObjectID()
	}
	cp := *m
	r.members[m.NIM] = &cp
	return nil
}

func (r *testMemberRepo) Update(_ context.Context, nim string, m *domain.Member) error {
	if _, ok := r.members[nim]; ok {
		cp := *m
		r.members[nim] = &cp
		return nil
	}
	return domain.NewNotFoundError("member")
}

func (r *testMemberRepo) Deactivate(_ context.Context, nim string) error {
	if m, ok := r.members[nim]; ok {
		m.IsActive = false
		return nil
	}
	return domain.NewNotFoundError("member")
}

func (r *testMemberRepo) Activate(_ context.Context, nim string) error {
	if m, ok := r.members[nim]; ok {
		m.IsActive = true
		return nil
	}
	return domain.NewNotFoundError("member")
}

func (r *testMemberRepo) DeletePermanent(_ context.Context, nim string) error {
	delete(r.members, nim)
	return nil
}

func (r *testMemberRepo) Summary(_ context.Context, _ time.Time) (*domain.MemberSummary, error) {
	return &domain.MemberSummary{}, nil
}

func (r *testMemberRepo) BulkUpsert(_ context.Context, _ []domain.Member) (int64, int64, error) {
	return 0, 0, nil
}

// setupAuthTestApp configures a test Fiber app with routes matching cmd/server/main.go
func setupAuthTestApp(t *testing.T) (*fiber.App, string) {
	const jwtSecret = "integration_test_secret_32_characters_long!"
	userRepo := newTestUserRepo()
	memberRepo := newTestMemberRepo()
	authSvc := service.NewAuthService(userRepo, memberRepo, jwtSecret, 24*time.Hour)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	// Seed Admin
	adminUser := &domain.User{
		ID:           primitive.NewObjectID(),
		NIM:          "000000001",
		Name:         "Admin Testing",
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
		Status:       domain.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = userRepo.Create(context.Background(), adminUser)

	// Seed Member
	memID := primitive.NewObjectID()
	memberRec := &domain.Member{
		ID:        memID,
		NIM:       "000000002",
		Name:      "Member Testing",
		Email:     "member@kampus.ac.id",
		IsActive:  true,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}
	_ = memberRepo.Create(context.Background(), memberRec)

	memberUser := &domain.User{
		ID:           primitive.NewObjectID(),
		NIM:          "000000002",
		Name:         "Member Testing",
		PasswordHash: string(hash),
		Role:         domain.RoleMember,
		Status:       domain.UserStatusActive,
		MemberID:     &memID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = userRepo.Create(context.Background(), memberUser)

	cfg := &config.Config{
		JWTSecret:   jwtSecret,
		JWTTTLHours: 24,
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	// Auth API
	api := app.Group("/api/v1")
	RegisterAuthRoutes(api, authSvc, cfg)

	// Admin API Protected Route Sample
	adminOnly := middleware.RequireRole(domain.RoleAdmin)
	authMiddleware := middleware.RequireAuth(jwtSecret)
	api.Get("/admin/data", authMiddleware, adminOnly, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "message": "admin only data"})
	})

	// Web Routes with redirects
	memberWebAuth := middleware.WebRequireAuth(jwtSecret, domain.RoleMember, domain.RoleAdmin)
	adminWebAuth := middleware.WebRequireAuth(jwtSecret, domain.RoleAdmin)
	guestWeb := middleware.WebRequireGuest(jwtSecret)

	app.Get("/login", guestWeb, func(c *fiber.Ctx) error {
		return c.SendString("login page")
	})
	app.Get("/belanja", memberWebAuth, func(c *fiber.Ctx) error {
		return c.SendString("shop page")
	})
	app.Get("/", adminWebAuth, func(c *fiber.Ctx) error {
		return c.SendString("admin dashboard page")
	})

	return app, jwtSecret
}

func TestAuthHandler_Login(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	t.Run("Admin login success returns 200 and redirect /", func(t *testing.T) {
		body := `{"nim": "000000001", "password": "password123"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		// Verify cookie set
		cookies := resp.Cookies()
		var authCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "auth_token" {
				authCookie = c
				break
			}
		}
		if authCookie == nil || authCookie.Value == "" {
			t.Error("expected auth_token cookie to be set")
		}

		// Verify body
		respBytes, _ := io.ReadAll(resp.Body)
		var envelope struct {
			Success bool `json:"success"`
			Data    struct {
				RedirectURL string `json:"redirect_url"`
				User        struct {
					Role string `json:"role"`
					NIM  string `json:"nim"`
				} `json:"user"`
			} `json:"data"`
		}
		_ = json.Unmarshal(respBytes, &envelope)

		if !envelope.Success {
			t.Errorf("expected success: true, got false")
		}
		if envelope.Data.RedirectURL != "/" {
			t.Errorf("expected redirect_url '/', got %s", envelope.Data.RedirectURL)
		}
		if envelope.Data.User.Role != "admin" {
			t.Errorf("expected role 'admin', got %s", envelope.Data.User.Role)
		}
	})

	t.Run("Member login success returns 200 and redirect /belanja", func(t *testing.T) {
		body := `{"nim": "000000002", "password": "password123"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		respBytes, _ := io.ReadAll(resp.Body)
		var envelope struct {
			Success bool `json:"success"`
			Data    struct {
				RedirectURL string `json:"redirect_url"`
				User        struct {
					Role string `json:"role"`
				} `json:"user"`
			} `json:"data"`
		}
		_ = json.Unmarshal(respBytes, &envelope)

		if envelope.Data.RedirectURL != "/belanja" {
			t.Errorf("expected redirect_url '/belanja', got %s", envelope.Data.RedirectURL)
		}
		if envelope.Data.User.Role != "member" {
			t.Errorf("expected role 'member', got %s", envelope.Data.User.Role)
		}
	})

	t.Run("Wrong password returns 401 Unauthorized", func(t *testing.T) {
		body := `{"nim": "000000001", "password": "incorrectpassword"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Missing credentials returns 422", func(t *testing.T) {
		body := `{"nim": "", "password": ""}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d", resp.StatusCode)
		}
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify cookie cleared (MaxAge < 0 or empty)
	cookies := resp.Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "auth_token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatal("expected auth_token cookie header in response")
	}
	if authCookie.Value != "" && authCookie.MaxAge > 0 {
		t.Errorf("expected auth_token to be cleared, got value=%s, maxAge=%d", authCookie.Value, authCookie.MaxAge)
	}
}

func TestAuthHandler_Me(t *testing.T) {
	app, jwtSecret := setupAuthTestApp(t)

	t.Run("Unauthenticated GET /me returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Authenticated GET /me returns 200 and profile", func(t *testing.T) {
		token, _ := security.GenerateToken(jwtSecret, security.JWTClaims{
			NIM:    "000000001",
			UserID: primitive.NewObjectID().Hex(),
			Role:   "admin",
		}, 1*time.Hour)

		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		respBytes, _ := io.ReadAll(resp.Body)
		var envelope struct {
			Success bool `json:"success"`
			Data    struct {
				NIM  string `json:"nim"`
				Role string `json:"role"`
			} `json:"data"`
		}
		_ = json.Unmarshal(respBytes, &envelope)

		if !envelope.Success || envelope.Data.NIM != "000000001" {
			t.Errorf("expected NIM 000000001, got %+v", envelope.Data)
		}
	})
}

func TestRoleBasedAuthorization(t *testing.T) {
	app, jwtSecret := setupAuthTestApp(t)

	adminToken, _ := security.GenerateToken(jwtSecret, security.JWTClaims{
		NIM:    "000000001",
		UserID: primitive.NewObjectID().Hex(),
		Role:   "admin",
	}, 1*time.Hour)
	memberToken, _ := security.GenerateToken(jwtSecret, security.JWTClaims{
		NIM:    "000000002",
		UserID: primitive.NewObjectID().Hex(),
		Role:   "member",
	}, 1*time.Hour)

	t.Run("Member accessing admin API returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/data", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: memberToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected status 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("Admin accessing admin API returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/data", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: adminToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("Unauthenticated user accessing admin web page redirects to /login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
			t.Fatalf("expected redirect status, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("expected redirect to /login, got %s", loc)
		}
	})

	t.Run("Member accessing admin web page redirects to /belanja", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: memberToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
			t.Fatalf("expected redirect status, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/belanja" {
			t.Errorf("expected redirect to /belanja, got %s", loc)
		}
	})

	t.Run("Logged-in admin visiting /login redirects to /", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: adminToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		loc := resp.Header.Get("Location")
		if loc != "/" {
			t.Errorf("expected redirect to /, got %s", loc)
		}
	})

	t.Run("Logged-in member visiting /login redirects to /belanja", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: memberToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		loc := resp.Header.Get("Location")
		if loc != "/belanja" {
			t.Errorf("expected redirect to /belanja, got %s", loc)
		}
	})
}

// Order Ownership Security Tests
type stubOrderSvc struct {
	createdReq *domain.CreateOrderRequest
}

func (s *stubOrderSvc) Create(_ context.Context, req *domain.CreateOrderRequest) (*domain.Order, error) {
	s.createdReq = req
	return &domain.Order{OrderNo: "ORD-TEST-001", MemberNIM: req.MemberNIM}, nil
}
func (s *stubOrderSvc) List(_ context.Context, _ domain.OrderFilter) ([]domain.Order, int64, error) {
	return nil, 0, nil
}
func (s *stubOrderSvc) GetByOrderNo(_ context.Context, _ string) (*domain.Order, error) {
	return &domain.Order{OrderNo: "ORD-TEST-001", MemberNIM: "000000002"}, nil
}
func (s *stubOrderSvc) Cancel(_ context.Context, _ string) error {
	return nil
}

func TestMemberOrderOwnershipSecurity(t *testing.T) {
	const jwtSecret = "integration_test_secret_32_characters_long!"
	orderSvc := &stubOrderSvc{}

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	api := app.Group("/api/v1")
	authMiddleware := middleware.RequireAuth(jwtSecret)
	RegisterOrderRoutes(api, orderSvc, authMiddleware)

	memberToken, _ := security.GenerateToken(jwtSecret, security.JWTClaims{
		NIM:    "000000002",
		UserID: primitive.NewObjectID().Hex(),
		Role:   "member",
	}, 1*time.Hour)

	t.Run("Member cannot create order for another member NIM (403)", func(t *testing.T) {
		body := `{"nim": "999999999", "channel": "ONLINE", "metode_pembayaran": "QRIS", "items": [{"kode_barang": "BRG-1", "qty": 1}]}`
		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: memberToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when member orders with foreign NIM, got %d", resp.StatusCode)
		}
	})

	t.Run("Member ordering with own NIM succeeds", func(t *testing.T) {
		body := `{"nim": "000000002", "channel": "ONLINE", "metode_pembayaran": "QRIS", "items": [{"kode_barang": "BRG-1", "qty": 1}]}`
		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: memberToken})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200/201 when member orders with own NIM, got %d", resp.StatusCode)
		}
	})
}
