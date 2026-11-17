package handler

import "github.com/gofiber/fiber/v2"

// RegisterOrderPages mendaftarkan halaman web (HTML) untuk modul pesanan.
// Rute API pesanan ada di RegisterOrderRoutes; fungsi ini hanya untuk halaman.
// Dipanggil dari main.go: handler.RegisterOrderPages(app)
//
// GET /orders/new -> form pemesanan kasir (PRD, Halaman 4).
// Catatan: GET /orders sudah didaftarkan di main.go dan menampilkan halaman yang sama.
func RegisterOrderPages(app *fiber.App) {
	app.Get("/orders/new", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/orders/new.html")
	})
}
