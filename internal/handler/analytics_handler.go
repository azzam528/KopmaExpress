package handler

import (
	"strconv"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"

	"github.com/gofiber/fiber/v2"
)

type AnalyticsHandler struct {
	service domain.AnalyticsService
}

func NewAnalyticsHandler(service domain.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) RegisterRoutes(api fiber.Router, middlewares ...fiber.Handler) {
	group := api.Group("/analytics")
	for _, m := range middlewares {
		if m != nil {
			group.Use(m)
		}
	}
	group.Get("/top-selling", h.GetTopSelling)
	group.Get("/top-stock", h.GetTopStock)
	group.Get("/summary", h.GetSummary)
	group.Get("/monthly-trend", h.GetMonthlyTrend)
}

func (h *AnalyticsHandler) GetTopSelling(c *fiber.Ctx) error {
	limit := queryInt(c, "limit", 10)
	from, to := c.Query("from"), c.Query("to")

	var (
		results []domain.TopSellingProduct
		err     error
		meta    fiber.Map
	)
	if from != "" || to != "" {
		if from == "" || to == "" {
			return httpx.WriteError(c, fiber.StatusUnprocessableEntity, domain.CodeValidationError,
				"Parameter from dan to harus diisi bersamaan", nil)
		}
		results, err = h.service.GetTopSellingRange(c.UserContext(), from, to, limit)
		meta = fiber.Map{"from": from, "to": to, "limit": boundedLimit(limit, 10, 50)}
	} else {
		period := c.Query("period", "6m")
		switch period {
		case "daily", "weekly", "monthly", "yearly":
			var window *domain.TopSellingWindow
			results, window, err = h.service.GetTopSellingByPeriod(
				c.UserContext(), period, c.Query("date"), c.Query("month"), limit)
			meta = fiber.Map{"period": period, "limit": boundedLimit(limit, 10, 50)}
			if window != nil {
				meta["from"], meta["to"] = window.From, window.To
			}
		default:
			results, err = h.service.GetTopSelling(c.UserContext(), period, limit)
			meta = fiber.Map{"period": period, "limit": boundedLimit(limit, 10, 50)}
		}
	}
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOKWithMeta(c, results, meta)
}

func (h *AnalyticsHandler) GetTopStock(c *fiber.Ctx) error {
	limit := queryInt(c, "limit", 10)
	results, totalValuation, err := h.service.GetTopStock(c.UserContext(), limit)
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOKWithMeta(c, results, fiber.Map{
		"limit":                boundedLimit(limit, 10, 50),
		"total_valuasi_gudang": totalValuation,
	})
}

func (h *AnalyticsHandler) GetSummary(c *fiber.Ctx) error {
	result, err := h.service.GetSummary(c.UserContext())
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOK(c, result)
}

func (h *AnalyticsHandler) GetMonthlyTrend(c *fiber.Ctx) error {
	code := c.Query("code")
	result, err := h.service.GetMonthlyTrend(c.UserContext(), code)
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOKWithMeta(c, result, fiber.Map{"kode_barang": code})
}

func queryInt(c *fiber.Ctx, key string, fallback int) int {
	value := c.Query(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func boundedLimit(value, fallback, maximum int) int {
	if value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
