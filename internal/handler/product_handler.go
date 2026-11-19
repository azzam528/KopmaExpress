package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

// ProductHandler melayani CRUD Master Barang (PRD §6.1).
type ProductHandler struct {
	svc domain.ProductService
}

// RegisterProductRoutes mendaftarkan rute /products.
// Rute pembacaan (GET) terbuka untuk katalog belanja dan kasir,
// sedangkan rute mutasi (POST, PUT, DELETE) dilindungi middleware admin.
func RegisterProductRoutes(api fiber.Router, svc domain.ProductService, adminMiddlewares ...fiber.Handler) {
	h := &ProductHandler{svc: svc}

	g := api.Group("/products")
	g.Get("/", h.List)
	g.Get("/:code", h.Get)

	var mws []fiber.Handler
	for _, m := range adminMiddlewares {
		if m != nil {
			mws = append(mws, m)
		}
	}

	postHandlers := append(append([]fiber.Handler{}, mws...), h.Create)
	putHandlers := append(append([]fiber.Handler{}, mws...), h.Update)
	deleteHandlers := append(append([]fiber.Handler{}, mws...), h.Delete)

	g.Post("/", postHandlers...)
	g.Put("/:code", putHandlers...)
	g.Delete("/:code", deleteHandlers...)
}

// GET /products?q=&category=&sort=&page=1&limit=20
// data = array barang aktif; meta = {page, limit, total, total_pages}.
// Bentuk array di `data` dipertahankan karena halaman pesanan (pemilih barang)
// juga memakai endpoint ini.
func (h *ProductHandler) List(c *fiber.Ctx) error {
	f := domain.ProductFilter{
		Query:    c.Query("q"),
		Category: c.Query("category"),
		Sort:     c.Query("sort"),
		Page:     c.QueryInt("page", 1),
		Limit:    c.QueryInt("limit", 20),
	}

	products, total, err := h.svc.List(c.UserContext(), f)
	if err != nil {
		return err
	}

	// samakan dengan normalisasi di service supaya meta sesuai data yang dikembalikan
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)

	return httpx.WriteOKWithMeta(c, products, fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GET /products/:code
func (h *ProductHandler) Get(c *fiber.Ctx) error {
	p, err := h.svc.GetByCode(c.UserContext(), c.Params("code"))
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, p)
}

// POST /products  -> 201
// Error (422 validasi, 409 kode ganda) dikembalikan apa adanya;
// middleware.ErrorHandler yang mengubahnya jadi envelope JSON standar.
func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	p, err := h.svc.Create(c.UserContext(), &req)
	if err != nil {
		return err
	}
	return httpx.WriteCreated(c, p)
}

// PUT /products/:code
// Parsial: field yang tidak dikirim tidak diubah. Kode barang tidak bisa diubah.
func (h *ProductHandler) Update(c *fiber.Ctx) error {
	var req domain.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	p, err := h.svc.Update(c.UserContext(), c.Params("code"), &req)
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, p)
}

// DELETE /products/:code  (soft delete: is_active=false)
func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	code := c.Params("code")
	if err := h.svc.Delete(c.UserContext(), code); err != nil {
		return err
	}
	return httpx.WriteOK(c, fiber.Map{
		"kode_barang": strings.ToUpper(strings.TrimSpace(code)),
		"is_active":   false,
	})
}