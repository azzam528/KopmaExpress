package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

// MemberSearcher dipenuhi oleh mongo.MemberLookupRepo.
type MemberSearcher interface {
	Search(ctx context.Context, q string, limit int) ([]domain.Member, error)
}

// memberOption hanya memuat field yang dibutuhkan pemilih NIM. Nomor telepon
// sengaja tidak ikut dikirim.
type memberOption struct {
	NIM      string `json:"nim"`
	Name     string `json:"nama"`
	Unit     string `json:"unit"`
	IsActive bool   `json:"is_active"`
}

// RegisterMemberLookupRoutes mendaftarkan GET /members?q=&limit= (SEMENTARA,
// untuk pemilih NIM di halaman pesanan). Bila Orang 1 sudah mendaftarkan
// GET /members sendiri, HAPUS pemanggilan fungsi ini di main.go agar tidak
// ada rute ganda.
func RegisterMemberLookupRoutes(api fiber.Router, s MemberSearcher) {
	api.Get("/members", func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		list, err := s.Search(c.UserContext(), q, c.QueryInt("limit", 20))
		if err != nil {
			return err
		}
		out := make([]memberOption, 0, len(list))
		for _, m := range list {
			out = append(out, memberOption{NIM: m.NIM, Name: m.Name, Unit: m.Unit, IsActive: m.IsActive})
		}
		return httpx.WriteOK(c, out)
	})
}
