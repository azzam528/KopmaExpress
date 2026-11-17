package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

type MemberHandler struct {
	svc domain.MemberService
}

// RegisterMemberRoutes mendaftarkan seluruh rute anggota di bawah /members.
// Dipanggil dari main.go: handler.RegisterMemberRoutes(api, memberSvc, authMiddleware)
// "/summary" HARUS didaftarkan sebelum "/:nim" agar tidak dianggap NIM.
func RegisterMemberRoutes(api fiber.Router, svc domain.MemberService, middlewares ...fiber.Handler) {
	h := &MemberHandler{svc: svc}

	g := api.Group("/members")
	for _, m := range middlewares {
		if m != nil {
			g.Use(m)
		}
	}

	g.Get("/", h.List)
	g.Get("/summary", h.Summary)
	g.Post("/", h.Create)
	g.Get("/:nim", h.Get)
	g.Put("/:nim", h.Update)
	g.Delete("/:nim", h.Deactivate)
	g.Post("/:nim/activate", h.Activate)
}

// GET /members?q=&status=&page=&limit=
func (h *MemberHandler) List(c *fiber.Ctx) error {
	f := domain.MemberFilter{
		Query:  c.Query("q"),
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 20),
	}

	members, total, err := h.svc.List(c.UserContext(), f)
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

	return httpx.WriteOKWithMeta(c, members, fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GET /members/summary
func (h *MemberHandler) Summary(c *fiber.Ctx) error {
	s, err := h.svc.Summary(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, s)
}

// GET /members/:nim
func (h *MemberHandler) Get(c *fiber.Ctx) error {
	m, err := h.svc.GetByNIM(c.UserContext(), c.Params("nim"))
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, m)
}

// POST /members
func (h *MemberHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	m, err := h.svc.Create(c.UserContext(), &req)
	if err != nil {
		return err
	}
	return httpx.WriteCreated(c, m)
}

// PUT /members/:nim
func (h *MemberHandler) Update(c *fiber.Ctx) error {
	var req domain.UpdateMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	m, err := h.svc.Update(c.UserContext(), c.Params("nim"), &req)
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, m)
}

// DELETE /members/:nim                  -> menonaktifkan (data & riwayat pesanan tetap ada)
// DELETE /members/:nim?permanent=true   -> hapus permanen; hanya untuk anggota
// nonaktif yang belum punya pesanan (selain itu 409).
func (h *MemberHandler) Deactivate(c *fiber.Ctx) error {
	nim := c.Params("nim")

	if strings.EqualFold(strings.TrimSpace(c.Query("permanent")), "true") {
		if err := h.svc.DeletePermanent(c.UserContext(), nim); err != nil {
			return err
		}
		return httpx.WriteOK(c, fiber.Map{"nim": strings.TrimSpace(nim), "deleted": true})
	}

	if err := h.svc.Delete(c.UserContext(), nim); err != nil {
		return err
	}
	return httpx.WriteOK(c, fiber.Map{"nim": strings.TrimSpace(nim), "is_active": false})
}

// POST /members/:nim/activate
func (h *MemberHandler) Activate(c *fiber.Ctx) error {
	nim := c.Params("nim")
	if err := h.svc.Activate(c.UserContext(), nim); err != nil {
		return err
	}
	return httpx.WriteOK(c, fiber.Map{"nim": strings.TrimSpace(nim), "is_active": true})
}
