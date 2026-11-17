package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

// ProductSearcher dipenuhi oleh mongo.ProductLookupRepo.
type ProductSearcher interface {
	Search(ctx context.Context, q string, limit int) ([]domain.Product, error)
}

// RegisterProductLookupRoutes mendaftarkan GET /products?q=&limit= (SEMENTARA,
// untuk pemilih barang di halaman pesanan). Bila Orang 1 sudah mendaftarkan
// GET /products sendiri, HAPUS pemanggilan fungsi ini di main.go agar tidak
// ada rute ganda.
func RegisterProductLookupRoutes(api fiber.Router, s ProductSearcher) {
	api.Get("/products", func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		list, err := s.Search(c.UserContext(), q, c.QueryInt("limit", 20))
		if err != nil {
			return err
		}
		return httpx.WriteOK(c, list)
	})
}
