package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"

	"chawy-erp-api/database"
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/router"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables from system")
	}

	// Connect to database
	database.ConnectDB()
	if os.Getenv("CLEAN_DB") == "true" {
		database.CleanMockData()
	}
	database.SeedData()
	startIntegrityScheduler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New(fiber.Config{
		AppName: "Chawy ERP API v2",
	})

	// CORS configuration
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000,http://localhost:8082"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))
	app.Use(middleware.StandardizeJSONResponse)

	// Public company assets (for example the invoice logo) are served from
	// api/image. The directory can be overridden for deployments via
	// ERP_IMAGE_DIR, while keeping the local default simple and predictable.
	imageDir := os.Getenv("ERP_IMAGE_DIR")
	if imageDir == "" {
		imageDir = "./image"
	}
	imageDir = filepath.Clean(imageDir)
	app.Static("/api/images", imageDir, fiber.Static{Browse: false, Compress: false})

	// Health Check Route
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// Setup API Routes
	router.SetupRoutes(app)
	startTiktokSyncScheduler(port)

	log.Printf("Starting server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func startTiktokSyncScheduler(port string) {
	minutes, _ := strconv.Atoi(os.Getenv("TIKTOK_SYNC_INTERVAL_MINUTES"))
	token := os.Getenv("TIKTOK_SYNC_TOKEN")
	if minutes <= 0 || token == "" {
		return
	}
	interval := time.Duration(minutes) * time.Minute
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/api/tiktok/orders/sync?days=1", nil)
			if err != nil {
				log.Printf("TikTok sync scheduler request error: %v", err)
				continue
			}
			req.Header.Set("Authorization", "Bearer "+token)
			response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
			if err != nil {
				log.Printf("TikTok sync scheduler failed: %v", err)
				continue
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode >= 400 {
				log.Printf("TikTok sync scheduler returned %d: %s", response.StatusCode, string(body))
			}
		}
	}()
}

func startIntegrityScheduler() {
	if os.Getenv("DISABLE_INTEGRITY_SCHEDULER") == "true" {
		return
	}
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
			if !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
			time.Sleep(time.Until(next))
			if run, err := handlers.RunIntegrityChecks("Nightly Scheduler"); err != nil {
				log.Printf("Nightly integrity check failed: %v", err)
			} else {
				log.Printf("Nightly integrity check %s completed with %d issue(s)", run.Code, run.IssueCount)
			}
		}
	}()
}
