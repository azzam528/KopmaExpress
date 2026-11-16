package handler

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"smartcoop/internal/domain"
	"smartcoop/internal/etl"
	"smartcoop/internal/httpx"
)

const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type ImportHandler struct {
	svc domain.ETLService
}

func NewImportHandler(svc domain.ETLService) *ImportHandler {
	return &ImportHandler{svc: svc}
}

func (h *ImportHandler) Register(r fiber.Router) {
	r.Post("/import/excel", h.Import)
	r.Get("/import/jobs", h.ListJobs)
	r.Get("/import/jobs/:id", h.GetJob)
	r.Get("/export/dummy-excel", h.DummyExcel)
}

func (h *ImportHandler) Import(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return httpx.WriteError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "field 'file' wajib diisi (multipart/form-data)", nil)
	}
	if !strings.EqualFold(filepath.Ext(fh.Filename), ".xlsx") {
		return httpx.WriteError(c, fiber.StatusUnsupportedMediaType, "UNSUPPORTED_FILE", "hanya file .xlsx yang didukung", nil)
	}

	file, err := fh.Open()
	if err != nil {
		return httpx.WriteError(c, fiber.StatusInternalServerError, "INTERNAL", "gagal membuka file upload", nil)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return httpx.WriteError(c, fiber.StatusInternalServerError, "INTERNAL", "gagal membaca file upload", nil)
	}

	job, err := h.svc.StartImport(c.UserContext(), fh.Filename, data, c.FormValue("stock_mode"))
	if err != nil {
		return writeImportError(c, err)
	}
	return httpx.WriteAccepted(c, fiber.Map{"job_id": job.ID.Hex(), "status": job.Status})
}

func (h *ImportHandler) GetJob(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return httpx.WriteError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "job id tidak valid", nil)
	}
	job, err := h.svc.GetJob(c.UserContext(), id)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return httpx.WriteError(c, fiber.StatusInternalServerError, "INTERNAL", "gagal mengambil status import", nil)
	}
	if err != nil || job == nil {
		return httpx.WriteError(c, fiber.StatusNotFound, "NOT_FOUND", "job import tidak ditemukan", nil)
	}
	return httpx.WriteOK(c, job)
}

func (h *ImportHandler) ListJobs(c *fiber.Ctx) error {
	page := max(c.QueryInt("page", 1), 1)
	limit := min(max(c.QueryInt("limit", 20), 1), 100)

	jobs, total, err := h.svc.ListJobs(c.UserContext(), page, limit)
	if err != nil {
		return httpx.WriteError(c, fiber.StatusInternalServerError, "INTERNAL", "gagal mengambil riwayat import", nil)
	}
	if jobs == nil {
		jobs = []domain.ImportJob{}
	}
	return httpx.WriteOKWithMeta(c, jobs, fiber.Map{"page": page, "limit": limit, "total": total})
}

func (h *ImportHandler) DummyExcel(c *fiber.Ctx) error {
	data, err := h.svc.GenerateDummyExcel(
		c.QueryInt("items", 300),
		c.QueryInt("months", 6),
		c.QueryBool("dirty", true),
		int64(c.QueryInt("seed", 42)),
	)
	if err != nil {
		return httpx.WriteError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
	}

	c.Set(fiber.HeaderContentType, xlsxContentType)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="data_koperasi_dummy.xlsx"`)
	return c.Send(data)
}

func writeImportError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, etl.ErrInvalidStockMode), errors.Is(err, etl.ErrMissingSheet):
		return httpx.WriteError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, etl.ErrInvalidWorkbook):
		return httpx.WriteError(c, fiber.StatusUnsupportedMediaType, "UNSUPPORTED_FILE", err.Error(), nil)
	default:
		return httpx.WriteError(c, fiber.StatusInternalServerError, "INTERNAL", "gagal memulai import", nil)
	}
}