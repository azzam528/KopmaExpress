package handler

import (
	"errors"
	"strings"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"

	"github.com/gofiber/fiber/v2"
)

type ForecastHandler struct {
	service domain.ForecastService
}

func NewForecastHandler(service domain.ForecastService) *ForecastHandler {
	return &ForecastHandler{service: service}
}

func (h *ForecastHandler) RegisterRoutes(api fiber.Router) {
	api.Post("/forecast/recalculate", h.Recalculate)
	api.Get("/forecast/summary", h.GetSummary)
	api.Get("/forecast", h.List)
	api.Get("/forecast/", h.List)
	api.Get("/forecast/:code", h.GetByProductCode)
}

func (h *ForecastHandler) Recalculate(c *fiber.Ctx) error {
	if err := h.service.Recalculate(c.UserContext()); err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOK(c, fiber.Map{"message": "Forecast berhasil dihitung ulang"})
}

func (h *ForecastHandler) List(c *fiber.Ctx) error {
	filter := domain.ForecastFilter{
		Status: domain.RestockStatus(strings.ToUpper(strings.TrimSpace(c.Query("status")))),
		Query:  strings.TrimSpace(c.Query("q")),
		Page:   queryInt(c, "page", 1),
		Limit:  queryInt(c, "limit", 20),
		Sort:   strings.TrimSpace(c.Query("sort", "restock_qty")),
	}

	results, total, err := h.service.List(c.UserContext(), filter)
	if err != nil {
		return writeHandlerError(c, err)
	}

	meta := fiber.Map{
		"page":  filter.Page,
		"limit": boundedLimit(filter.Limit, 20, 100),
		"total": total,
	}
	if len(results) > 0 {
		meta["target_month"] = results[0].TargetMonth
		meta["calculated_at"] = results[0].CalculatedAt
	}
	return httpx.WriteOKWithMeta(c, results, meta)
}

func (h *ForecastHandler) GetSummary(c *fiber.Ctx) error {
	result, err := h.service.GetSummary(c.UserContext())
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOK(c, result)
}

func (h *ForecastHandler) GetByProductCode(c *fiber.Ctx) error {
	result, err := h.service.GetByProductCode(c.UserContext(), strings.TrimSpace(c.Params("code")))
	if err != nil {
		return writeHandlerError(c, err)
	}
	return httpx.WriteOK(c, result)
}

func writeHandlerError(c *fiber.Ctx, err error) error {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return httpx.WriteError(c, appErr.HTTPStatus, appErr.Code, appErr.Message, appErr.Details)
	}
	return httpx.WriteError(c, fiber.StatusInternalServerError, domain.CodeInternal, "Terjadi kesalahan pada server", nil)
}
