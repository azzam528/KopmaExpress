package middleware_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
	"smartcoop/internal/middleware"
)

func TestErrorHandler(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Get("/test-not-found", func(c *fiber.Ctx) error {
		return domain.NewNotFoundError("Data tidak ditemukan")
	})

	app.Get("/test-validation", func(c *fiber.Ctx) error {
		return domain.NewValidationError("Input tidak valid", domain.ValidationErrorDetail{
			Field:   "kode_barang",
			Message: "harus diisi",
		})
	})

	app.Get("/test-fiber-404", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "Route not found")
	})

	// Test 1: Domain NotFoundError
	req1 := httptest.NewRequest(http.MethodGet, "/test-not-found", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("unexpected error on req1: %v", err)
	}
	if resp1.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp1.StatusCode)
	}
	body1, _ := io.ReadAll(resp1.Body)
	var env1 httpx.APIResponse
	if err := json.Unmarshal(body1, &env1); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if env1.Success != false || env1.Error == nil || env1.Error.Code != domain.CodeNotFound {
		t.Errorf("unexpected body1: %s", string(body1))
	}

	// Test 2: Domain ValidationError
	req2 := httptest.NewRequest(http.MethodGet, "/test-validation", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("unexpected error on req2: %v", err)
	}
	if resp2.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	var env2 httpx.APIResponse
	if err := json.Unmarshal(body2, &env2); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if env2.Success != false || env2.Error == nil || env2.Error.Code != domain.CodeValidationError {
		t.Errorf("unexpected body2: %s", string(body2))
	}

	// Test 3: Standard Fiber 404
	req3 := httptest.NewRequest(http.MethodGet, "/test-fiber-404", nil)
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("unexpected error on req3: %v", err)
	}
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp3.StatusCode)
	}
}
