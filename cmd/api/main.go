package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"chawy-erp-api/config"
	"chawy-erp-api/internal/delivery/http/handler"
	"chawy-erp-api/internal/delivery/http/middleware"
	"chawy-erp-api/internal/delivery/http/route"
	domainAuth "chawy-erp-api/internal/domain/auth"
	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainSettings "chawy-erp-api/internal/domain/settings"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/internal/repository/postgres"
	usecaseAuth "chawy-erp-api/internal/usecase/auth"
	usecaseBundle "chawy-erp-api/internal/usecase/bundle"
	usecaseCustomer "chawy-erp-api/internal/usecase/customer"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"
	usecaseReport "chawy-erp-api/internal/usecase/report"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	usecaseTikTok "chawy-erp-api/internal/usecase/tiktok"
	"chawy-erp-api/pkg/database"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Database Connection
	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to database: %v", err)
	}

	// 3. Database Auto Migration
	if err := db.AutoMigrate(
		&domainAuth.User{},
		&domainSKU.SKU{},
		&domainBundle.BundleItem{},
		&domainStock.Stock{},
		&domainStock.StockMovement{},
		&domainCustomer.Customer{},
		&domainOrder.Order{},
		&domainOrder.OrderItem{},
		&domainPurchasing.Supplier{},
		&domainPurchasing.PurchaseOrder{},
		&domainPurchasing.POItem{},
		&domainPurchasing.GoodsReceive{},
		&domainPurchasing.GoodsReceiveItem{},
		&domainQuotation.Quotation{},
		&domainQuotation.QuotationLine{},
		&domainInvoice.Invoice{},
		&domainTikTok.TiktokConnection{},
		&domainTikTok.TiktokOAuthState{},
		&domainTikTok.TiktokWebhookEvent{},
		&domainTikTok.TiktokSyncRun{},
		&domainTikTok.TiktokOrder{},
		&domainTikTok.TiktokOrderItem{},
		&domainTikTok.SKUMapping{},
		&domainTikTok.SyncLog{},
		&domainSettings.CompanySettings{},
		&domainSettings.NotificationSettings{},
		&domainSettings.ModuleSettings{},
		&domainSettings.LivePayrollSettings{},
	); err != nil {
		log.Printf("[WARN] AutoMigrate warning: %v", err)
	}

	// 3.0 Column compatibility migration for stocks and stock_movements
	_ = db.Exec(`
		DO $$ 
		BEGIN 
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='stocks' AND column_name='sk_uid') THEN
				ALTER TABLE stocks ADD COLUMN IF NOT EXISTS sku_id bigint;
				ALTER TABLE stocks ALTER COLUMN sk_uid DROP NOT NULL;
				UPDATE stocks SET sku_id = sk_uid WHERE sku_id IS NULL AND sk_uid IS NOT NULL;
				UPDATE stocks SET sk_uid = sku_id WHERE sk_uid IS NULL AND sku_id IS NOT NULL;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='stock_movements' AND column_name='sk_uid') THEN
				ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS sku_id bigint;
				ALTER TABLE stock_movements ALTER COLUMN sk_uid DROP NOT NULL;
				UPDATE stock_movements SET sku_id = sk_uid WHERE sku_id IS NULL AND sk_uid IS NOT NULL;
				UPDATE stock_movements SET sk_uid = sku_id WHERE sk_uid IS NULL AND sku_id IS NOT NULL;
			END IF;
		END $$;
	`).Error

	// 3.1 Seed initial admin user if not exists
	seedDefaultAdmin(db)

	// 4. Dependency Injection - Repositories
	authRepo := postgres.NewAuthRepository(db)
	skuRepo := postgres.NewSKURepository(db)
	bundleRepo := postgres.NewBundleRepository(db)
	stockRepo := postgres.NewStockRepository(db)
	customerRepo := postgres.NewCustomerRepository(db)
	orderRepo := postgres.NewOrderRepository(db)
	purchasingRepo := postgres.NewPurchasingRepository(db)
	invoiceRepo := postgres.NewInvoiceRepository(db)
	tiktokRepo := postgres.NewTikTokRepository(db)
	settingsRepo := postgres.NewSettingsRepository(db)

	// 5. Dependency Injection - Usecases
	authUsecase := usecaseAuth.NewAuthUsecase(authRepo, cfg.JWTSecret, cfg.JWTExpHours)
	skuUsecase := usecaseSKU.NewSKUUsecase(skuRepo)
	bundleUsecase := usecaseBundle.NewBundleUsecase(bundleRepo, skuRepo, stockRepo)
	stockUsecase := usecaseStock.NewStockUsecase(stockRepo)
	customerUsecase := usecaseCustomer.NewCustomerUsecase(customerRepo)
	orderUsecase := usecaseOrder.NewOrderUsecase(db, orderRepo, skuRepo, bundleRepo, stockRepo)
	purchasingUsecase := usecasePurchasing.NewPurchasingUsecase(db, purchasingRepo, skuRepo, stockRepo)
	invoiceUsecase := usecaseInvoice.NewInvoiceUsecase(invoiceRepo, orderRepo)
	reportUsecase := usecaseReport.NewReportUsecase(db)
	tiktokUsecase := usecaseTikTok.NewTikTokUsecase(cfg, db, tiktokRepo, orderRepo, skuRepo, bundleRepo, stockRepo)
	settingsUsecase := usecaseSettings.NewSettingsUsecase(settingsRepo)

	// 6. Dependency Injection - Handlers
	authHdl := handler.NewAuthHandler(authUsecase)
	skuHdl := handler.NewSKUHandler(skuUsecase)
	bundleHdl := handler.NewBundleHandler(bundleUsecase)
	stockHdl := handler.NewStockHandler(stockUsecase)
	customerHdl := handler.NewCustomerHandler(customerUsecase)
	orderHdl := handler.NewOrderHandler(orderUsecase)
	purchasingHdl := handler.NewPurchasingHandler(purchasingUsecase)
	invoiceHdl := handler.NewInvoiceHandler(invoiceUsecase)
	reportHdl := handler.NewReportHandler(reportUsecase)
	workspaceHdl := handler.NewWorkspaceHandler(db, orderUsecase)
	tiktokHdl := handler.NewTikTokHandler(tiktokUsecase)
	settingsHdl := handler.NewSettingsHandler(settingsUsecase)

	// 7. Initialize Fiber App
	app := fiber.New(fiber.Config{
		AppName:      "Chawy ERP API - Clean Architecture v2",
		ErrorHandler: middleware.GlobalErrorHandler,
	})

	// 8. Global Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: false,
	}))

	// 9. Register Routes
	route.RegisterRoutes(route.Config{
		App:               app,
		AuthHandler:       authHdl,
		SKUHandler:        skuHdl,
		BundleHandler:     bundleHdl,
		StockHandler:      stockHdl,
		CustomerHandler:   customerHdl,
		OrderHandler:      orderHdl,
		PurchasingHandler: purchasingHdl,
		InvoiceHandler:    invoiceHdl,
		ReportHandler:     reportHdl,
		WorkspaceHandler:  workspaceHdl,
		TikTokHandler:     tiktokHdl,
		SettingsHandler:   settingsHdl,
		JWTSecret:         cfg.JWTSecret,
	})

	// 9.1 Start Background TikTok Sync Scheduler
	startTiktokSyncScheduler(cfg, tiktokUsecase)

	// 10. Start Server
	log.Printf("[INFO] Server starting on port %s...", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("[FATAL] Server terminated unexpectedly: %v", err)
	}
}

func startTiktokSyncScheduler(cfg *config.Config, tiktokUsecase usecaseTikTok.Usecase) {
	intervalMinutes, _ := strconv.Atoi(cfg.TikTokSyncIntervalMinutes)
	if intervalMinutes <= 0 || cfg.TikTokAppKey == "" {
		return
	}
	interval := time.Duration(intervalMinutes) * time.Minute
	go func() {
		log.Printf("[INFO] Background TikTok sync scheduler started (interval: %v)", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			res, err := tiktokUsecase.SyncOrders(ctx, 1)
			if err != nil {
				log.Printf("[WARN] Background TikTok sync error: %v", err)
			} else {
				log.Printf("[INFO] Background TikTok sync finished: %d orders synced, %d stock deducted", res.Synced, res.StockDeducted)
			}
		}
	}()
}

func seedDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&domainAuth.User{}).Count(&count)
	if count == 0 {
		hashed, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("[WARN] Failed to hash admin password: %v", err)
			return
		}
		hashedStr := string(hashed)
		defaultUsers := []domainAuth.User{
			{
				Email:    "admin@example.com",
				Name:     "Admin System",
				Role:     "admin",
				Password: hashedStr,
			},
			{
				Email:    "admin@mail.com",
				Name:     "Admin Mail",
				Role:     "admin",
				Password: hashedStr,
			},
		}
		for _, u := range defaultUsers {
			if err := db.Create(&u).Error; err != nil {
				log.Printf("[WARN] Failed to seed user %s: %v", u.Email, err)
			}
		}
		log.Println("[INFO] Successfully seeded default users: admin@example.com / admin@mail.com (password: admin123)")
	}
}
