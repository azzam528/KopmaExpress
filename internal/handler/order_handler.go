package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

// wib: tanggal tanpa jam pada filter (?from=2026-11-01) dianggap waktu WIB.
var wib = time.FixedZone("WIB", 7*60*60)

type OrderHandler struct {
	svc domain.OrderService
}

// RegisterOrderRoutes mendaftarkan seluruh rute pesanan di bawah /orders.
// Dipanggil dari main.go: handler.RegisterOrderRoutes(api, orderSvc)
func RegisterOrderRoutes(api fiber.Router, svc domain.OrderService) {
	h := &OrderHandler{svc: svc}

	g := api.Group("/orders")
	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/:order_no", h.GetByOrderNo)
	g.Post("/:order_no/cancel", h.Cancel)
}

// POST /orders
// Error (409 stok kurang, 422 validasi, 404 anggota/barang) dikembalikan apa adanya;
// middleware.ErrorHandler yang mengubahnya jadi envelope JSON standar.
func (h *OrderHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.NewValidationError("Body harus berupa JSON yang valid (Content-Type: application/json)")
	}

	order, err := h.svc.Create(c.UserContext(), &req)
	if err != nil {
		return err
	}
	return httpx.WriteCreated(c, order)
}

// GET /orders?nim=&from=&to=&status=&page=&limit=
func (h *OrderHandler) List(c *fiber.Ctx) error {
	f := domain.OrderFilter{
		MemberNIM: c.Query("nim"),
		Status:    domain.OrderStatus(c.Query("status")),
		Page:      c.QueryInt("page", 1),
		Limit:     c.QueryInt("limit", 20),
	}

	var details []domain.ValidationErrorDetail
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		if t, ok := parseTime(raw, false); ok {
			f.From = t
		} else {
			details = append(details, domain.ValidationErrorDetail{
				Field: "from", Message: "format harus YYYY-MM-DD atau RFC3339",
			})
		}
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		if t, ok := parseTime(raw, true); ok {
			f.To = t
		} else {
			details = append(details, domain.ValidationErrorDetail{
				Field: "to", Message: "format harus YYYY-MM-DD atau RFC3339",
			})
		}
	}
	if len(details) > 0 {
		return domain.NewValidationError("Parameter filter tidak valid", details...)
	}

	orders, total, err := h.svc.List(c.UserContext(), f)
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

	return httpx.WriteOKWithMeta(c, orders, fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GET /orders/:order_no
func (h *OrderHandler) GetByOrderNo(c *fiber.Ctx) error {
	order, err := h.svc.GetByOrderNo(c.UserContext(), c.Params("order_no"))
	if err != nil {
		return err
	}
	return httpx.WriteOK(c, order)
}

// POST /orders/:order_no/cancel
func (h *OrderHandler) Cancel(c *fiber.Ctx) error {
	orderNo := c.Params("order_no")
	if err := h.svc.Cancel(c.UserContext(), orderNo); err != nil {
		return err
	}
	return httpx.WriteOK(c, fiber.Map{
		"order_no": strings.TrimSpace(orderNo),
		"status":   domain.OrderCancelled,
	})
}

// parseTime menerima "YYYY-MM-DD" (WIB) atau RFC3339.
// Untuk batas akhir (to) berformat tanggal saja, hasilnya akhir hari tersebut
// agar pesanan pada hari itu ikut terhitung.
func parseTime(raw string, endOfDay bool) (*time.Time, bool) {
	if t, err := time.ParseInLocation("2006-01-02", raw, wib); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Millisecond)
		}
		return &t, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, true
	}
	return nil, false
}
