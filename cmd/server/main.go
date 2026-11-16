package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"smartcoop/internal/config"
	"smartcoop/internal/handler"
	"smartcoop/internal/httpx"
	"smartcoop/internal/middleware"
	mongorepo "smartcoop/internal/repository/mongo"
	"smartcoop/internal/service"
)

func main() {
	// 1. Load Configuration
	cfg := config.Load()
	log.Printf("[INIT] Starting KOPMA Express (Environment: %s, Port: %s, Workers: %d)\n",
		cfg.AppEnv, cfg.Port, cfg.Workers)

	// 2. Initialize MongoDB Connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var db *mongo.Database
	var client *mongo.Client
	var err error

	client, err = mongorepo.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Printf("[MONGO] Warning: MongoDB connection failed: %v. Running in degraded mode.\n", err)
	} else {
		db = client.Database(cfg.DBName)
		if idxErr := mongorepo.EnsureIndexes(context.Background(), db); idxErr != nil {
			log.Printf("[MONGO] Warning: Could not ensure indexes: %v\n", idxErr)
		}
	}

	// 3. Create Fiber App with limits and error handling
	app := fiber.New(fiber.Config{
		AppName:      "KOPMA Express v1.0",
		BodyLimit:    cfg.MaxUploadMB * 1024 * 1024,
		ErrorHandler: middleware.ErrorHandler,
	})

	// 4. Register Standard Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Role",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// 5. Static Files & Web Page Routes (PRD §6.7)
	app.Static("/static", "./web/static")

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})
	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.Redirect("/")
	})
	app.Get("/products", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/products.html")
	})
	app.Get("/members", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/members.html")
	})
	app.Get("/import", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/import.html")
	})
	app.Get("/orders", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/orders.html")
	})
	app.Get("/orders/new", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/orders.html")
	})
	app.Get("/forecast", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/forecast.html")
	})

	// 6. Base API Group
	api := app.Group("/api/v1")

	// Health Check Endpoint
	api.Get("/health", func(c *fiber.Ctx) error {
		dbStatus := "CONNECTED"
		if client == nil {
			dbStatus = "DISCONNECTED"
		} else {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer pingCancel()
			if pingErr := client.Ping(pingCtx, readpref.PrimaryPreferred()); pingErr != nil {
				dbStatus = "UNREACHABLE"
			}
		}

		return httpx.WriteOK(c, fiber.Map{
			"status":    "UP",
			"timestamp": time.Now().Format(time.RFC3339),
			"database":  dbStatus,
			"version":   "1.0.0",
		})
	})

		// Order routes (Orang 3)
	if db != nil {
		orderRepo := mongorepo.NewOrderRepo(client, db)
		productRepo := mongorepo.NewProductRepo(db)
		memberRepo := mongorepo.NewMemberRepo(db)
		orderSvc := service.NewOrderService(orderRepo, productRepo, memberRepo)
		handler.RegisterOrderRoutes(api, orderSvc)
	}

	// 7. Start Server in Goroutine
	addr := fmt.Sprintf(":%s", cfg.Port)
	go func() {
		log.Printf("[SERVER] KOPMA Express listening on %s\n", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("[SERVER] Listener stopped: %v\n", err)
		}
	}()

	// 9. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[SHUTDOWN] Gracefully shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Printf("[SHUTDOWN] Error during Fiber shutdown: %v\n", err)
	}

	if client != nil {
		log.Println("[SHUTDOWN] Disconnecting from MongoDB...")
		if err := client.Disconnect(shutdownCtx); err != nil {
			log.Printf("[SHUTDOWN] Error disconnecting MongoDB: %v\n", err)
		}
	}

	log.Println("[SHUTDOWN] Server exited cleanly.")
}
