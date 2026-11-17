package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/middleware"
)

// stubProductSvc mencatat input yang diterima handler dan mengembalikan hasil yang diset test.
type stubProductSvc struct {
	listFilter domain.ProductFilter
	listOut    []domain.Product
	listTotal  int64

	createIn  *domain.CreateProductRequest
	createErr error

	getErr error

	updateCode string
	updateIn   *domain.UpdateProductRequest

	deleted string
}

func (s *stubProductSvc) List(_ context.Context, f domain.ProductFilter) ([]domain.Product, int64, error) {
	s.listFilter = f
	return s.listOut, s.listTotal, nil
}

func (s *stubProductSvc) GetByCode(_ context.Context, code string) (*domain.Product, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &domain.Product{Code: code, Name: "Buku", IsActive: true}, nil
}

func (s *stubProductSvc) Create(_ context.Context, req *domain.CreateProductRequest) (*domain.Product, error) {
	s.createIn = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &domain.Product{Code: req.Code, Name: req.Name, Category: req.Category, Price: req.Price, Stock: req.Stock, IsActive: true}, nil
}

func (s *stubProductSvc) Update(_ context.Context, code string, req *domain.UpdateProductRequest) (*domain.Product, error) {
	s.updateCode, s.updateIn = code, req
	return &domain.Product{Code: code, IsActive: true}, nil
}

func (s *stubProductSvc) Delete(_ context.Context, code string) error {
	s.deleted = code
	return nil
}

func newProductApp(svc domain.ProductService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
	RegisterProductRoutes(app.Group("/api/v1"), svc)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("respons bukan JSON: %v (%s)", err, raw)
	}
	return resp.StatusCode, env
}

func errCode(env map[string]any) string {
	e, _ := env["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestProductHandler_Create_201AndEnvelope(t *testing.T) {
	svc := &stubProductSvc{}
	app := newProductApp(svc)

	status, env := doJSON(t, app, http.MethodPost, "/api/v1/products",
		`{"kode_barang":"BRG-0001","nama_barang":"Buku Tulis","kategori":"ATK","harga":3500,"stok":120}`)

	if status != http.StatusCreated {
		t.Fatalf("want 201, got %d", status)
	}
	if env["success"] != true || env["error"] != nil {
		t.Fatalf("envelope salah: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["kode_barang"] != "BRG-0001" || data["harga"] != float64(3500) || data["stok"] != float64(120) {
		t.Fatalf("data salah (nama field JSON harus kode_barang/harga/stok): %v", data)
	}
	if svc.createIn == nil || svc.createIn.Price != 3500 || svc.createIn.Stock != 120 {
		t.Fatalf("body tidak ter-parse ke request: %+v", svc.createIn)
	}
}

func TestProductHandler_Create_InvalidJSON422(t *testing.T) {
	app := newProductApp(&stubProductSvc{})

	status, env := doJSON(t, app, http.MethodPost, "/api/v1/products", `{"kode_barang":`)
	if status != http.StatusUnprocessableEntity || errCode(env) != domain.CodeValidationError {
		t.Fatalf("want 422/VALIDATION_ERROR, got %d/%s", status, errCode(env))
	}
}

func TestProductHandler_Create_DuplicatePassesThrough409(t *testing.T) {
	app := newProductApp(&stubProductSvc{createErr: domain.NewDuplicateError("Kode barang sudah digunakan")})

	status, env := doJSON(t, app, http.MethodPost, "/api/v1/products",
		`{"kode_barang":"BRG-1","nama_barang":"X","kategori":"ATK"}`)
	if status != http.StatusConflict || errCode(env) != domain.CodeDuplicate {
		t.Fatalf("want 409/DUPLICATE, got %d/%s", status, errCode(env))
	}
}

func TestProductHandler_List_ParsesQueryAndBuildsMeta(t *testing.T) {
	svc := &stubProductSvc{listOut: []domain.Product{{Code: "A"}, {Code: "B"}}, listTotal: 45}
	app := newProductApp(svc)

	status, env := doJSON(t, app, http.MethodGet,
		"/api/v1/products?q=buku&category=ATK&sort=stock_desc&page=2&limit=20", "")
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}

	f := svc.listFilter
	if f.Query != "buku" || f.Category != "ATK" || f.Sort != "stock_desc" || f.Page != 2 || f.Limit != 20 {
		t.Fatalf("query tidak ter-parse: %+v", f)
	}
	if arr, ok := env["data"].([]any); !ok || len(arr) != 2 {
		t.Fatalf("data harus berupa array berisi 2 item: %v", env["data"])
	}
	meta := env["meta"].(map[string]any)
	if meta["page"] != float64(2) || meta["limit"] != float64(20) || meta["total"] != float64(45) || meta["total_pages"] != float64(3) {
		t.Fatalf("meta salah: %v", meta)
	}
}

func TestProductHandler_List_MetaLimitIsClamped(t *testing.T) {
	app := newProductApp(&stubProductSvc{})

	_, env := doJSON(t, app, http.MethodGet, "/api/v1/products?limit=9999&page=0", "")
	meta := env["meta"].(map[string]any)
	if meta["limit"] != float64(100) || meta["page"] != float64(1) {
		t.Fatalf("meta harus mengikuti normalisasi service (page=1, limit=100): %v", meta)
	}
}

func TestProductHandler_Get_NotFound404(t *testing.T) {
	app := newProductApp(&stubProductSvc{getErr: domain.NewNotFoundError("Produk tidak ditemukan")})

	status, env := doJSON(t, app, http.MethodGet, "/api/v1/products/BRG-404", "")
	if status != http.StatusNotFound || errCode(env) != domain.CodeNotFound {
		t.Fatalf("want 404/NOT_FOUND, got %d/%s", status, errCode(env))
	}
}

func TestProductHandler_Update_PartialBodyKeepsNilPointers(t *testing.T) {
	svc := &stubProductSvc{}
	app := newProductApp(svc)

	status, _ := doJSON(t, app, http.MethodPut, "/api/v1/products/BRG-1", `{"stok":0}`)
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
	if svc.updateCode != "BRG-1" {
		t.Fatalf("kode harus diambil dari URL, got %q", svc.updateCode)
	}
	if svc.updateIn.Stock == nil || *svc.updateIn.Stock != 0 {
		t.Fatalf("stok 0 harus terkirim sebagai pointer non-nil: %+v", svc.updateIn)
	}
	if svc.updateIn.Price != nil {
		t.Fatal("harga tidak dikirim, pointer harus nil (tidak diubah)")
	}
}

func TestProductHandler_Delete_ReturnsInactive(t *testing.T) {
	svc := &stubProductSvc{}
	app := newProductApp(svc)

	status, env := doJSON(t, app, http.MethodDelete, "/api/v1/products/brg-9", "")
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
	if svc.deleted != "brg-9" {
		t.Fatalf("handler meneruskan kode apa adanya ke service, got %q", svc.deleted)
	}
	data := env["data"].(map[string]any)
	if data["kode_barang"] != "BRG-9" || data["is_active"] != false {
		t.Fatalf("data salah: %v", data)
	}
}