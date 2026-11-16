package handler

import "github.com/gofiber/fiber/v2"

// RegisterOrderPages menyajikan halaman web pemesanan (Orang 3).
// Dipanggil dari main.go: handler.RegisterOrderPages(app)
func RegisterOrderPages(app fiber.Router) {
	app.Get("/orders/new", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/orders_new.html")
	})
}
