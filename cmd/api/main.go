package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"chawy-erp-api/config"
	"chawy-erp-api/internal/delivery/http/handler"
	"chawy-erp-api/internal/delivery/http/middleware"
	"chawy-erp-api/internal/delivery/http/route"
	domainAuth "chawy-erp-api/internal/domain/auth"
	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainFinance "chawy-erp-api/internal/domain/finance"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainLive "chawy-erp-api/internal/domain/live"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSettings "chawy-erp-api/internal/domain/settings"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/internal/repository/postgres"
	usecaseAuth "chawy-erp-api/internal/usecase/auth"
	usecaseBundle "chawy-erp-api/internal/usecase/bundle"
	usecaseCustomer "chawy-erp-api/internal/usecase/customer"
	usecaseFinance "chawy-erp-api/internal/usecase/finance"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	usecaseLive "chawy-erp-api/internal/usecase/live"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"
	usecaseQuotation "chawy-erp-api/internal/usecase/quotation"
	usecaseReport "chawy-erp-api/internal/usecase/report"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"
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

	// FULL-02: refuse to start production without a real JWT secret
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[FATAL] Invalid configuration: %v", err)
	}

	// 2. Database Connection
	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to database: %v", err)
	}

	// 3. Database Auto Migration
	if err := db.AutoMigrate(
		&domainAuth.User{},
		&domainSKU.SKU{},
		&domainSKU.SKUAccessory{},
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
		&domainFinance.Account{},
		&domainFinance.AccountMapping{},
		&domainFinance.JournalEntry{},
		&domainFinance.JournalLine{},
		&domainFinance.Expense{},
		&domainLive.LiveSession{},
		&domainLive.ContentItem{},
	); err != nil {
		// Fail-closed: running on an incompatible schema causes partial data
		// failures at runtime; it is safer to refuse to start.
		log.Fatalf("[FATAL] AutoMigrate failed: %v", err)
	}

	// 3.0 Column compatibility migration for stocks and stock_movements
	if err := db.Exec(`
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
	`).Error; err != nil {
		log.Fatalf("[FATAL] Compatibility migration failed: %v", err)
	}

	// 3.1 Seed initial admin user only in development (FULL-01)
	seedDefaultAdmin(db, cfg.Environment)
	seedDefaultAccounts(db)

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
	txManager := database.NewTxManager(db)

	authUsecase := usecaseAuth.NewAuthUsecase(authRepo, cfg.JWTSecret, cfg.JWTExpHours)
	skuUsecase := usecaseSKU.NewSKUUsecase(skuRepo)
	bundleUsecase := usecaseBundle.NewBundleUsecase(bundleRepo, skuRepo, stockRepo)
	stockUsecase := usecaseStock.NewStockUsecaseWithTx(stockRepo, txManager)
	customerUsecase := usecaseCustomer.NewCustomerUsecase(customerRepo)
	orderUsecase := usecaseOrder.NewOrderUsecase(db, orderRepo, skuRepo, bundleRepo, stockRepo)
	purchasingUsecase := usecasePurchasing.NewPurchasingUsecaseWithTx(db, purchasingRepo, skuRepo, stockRepo, txManager)
	invoiceUsecase := usecaseInvoice.NewInvoiceUsecaseWithTx(invoiceRepo, orderRepo, txManager)
	reportUsecase := usecaseReport.NewReportUsecase(db)
	tiktokUsecase := usecaseTikTok.NewTikTokUsecase(cfg, db, tiktokRepo, orderRepo, skuRepo, bundleRepo, stockRepo)
	settingsUsecase := usecaseSettings.NewSettingsUsecase(settingsRepo)
	financeRepo := postgres.NewFinanceRepository(db)
	financeUsecase := usecaseFinance.NewFinanceUsecase(financeRepo, txManager)
	quotationRepo := postgres.NewQuotationRepository(db)
	quotationUsecase := usecaseQuotation.NewQuotationUsecaseWithStock(quotationRepo, skuRepo, orderRepo, txManager, stockRepo, bundleRepo)
	liveRepo := postgres.NewLiveRepository(db)
	liveUsecase := usecaseLive.NewLiveUsecase(db, liveRepo, settingsRepo, authRepo)

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
	financeHdl := handler.NewFinanceHandler(financeUsecase)
	workspaceHdl := handler.NewWorkspaceHandler(db, orderUsecase, quotationUsecase, stockUsecase, skuRepo)
	tiktokHdl := handler.NewTikTokHandler(tiktokUsecase)
	settingsHdl := handler.NewSettingsHandler(settingsUsecase)
	uploadHdl := handler.NewUploadHandler()
	liveHdl := handler.NewLiveHandler(liveUsecase)

	// 7. Initialize Fiber App
	app := fiber.New(fiber.Config{
		AppName:      "Chawy ERP API - Clean Architecture v2",
		ErrorHandler: middleware.GlobalErrorHandler,
		BodyLimit:    10 * 1024 * 1024, // 10MB for image uploads
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

	// Serve uploaded files statically
	app.Static("/uploads", "./uploads")

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
		FinanceHandler:    financeHdl,
		WorkspaceHandler:  workspaceHdl,
		TikTokHandler:     tiktokHdl,
		SettingsHandler:   settingsHdl,
		UploadHandler:     uploadHdl,
		LiveHandler:       liveHdl,
		JWTSecret:         cfg.JWTSecret,
		UserStatusLoader:  authRepo,
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

// seedDefaultAdmin creates development-only admin accounts when the users
// table is empty. FULL-01: never runs in production, never logs credentials.
func seedDefaultAdmin(db *gorm.DB, environment string) {
	if environment == "production" {
		log.Println("[INFO] Skipping default admin seed in production; use a bootstrap step to create the first owner account")
		return
	}

	var count int64
	db.Model(&domainAuth.User{}).Count(&count)
	if count > 0 {
		return
	}

	devPassword := os.Getenv("DEV_SEED_PASSWORD")
	if devPassword == "" {
		devPassword = "admin123"
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(devPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[WARN] Failed to hash admin password: %v", err)
		return
	}
	hashedStr := string(hashed)
	defaultUsers := []domainAuth.User{
		{
			Email: "admin@example.com",
			Name:  "Admin System",
			// Must be a role accepted by isValidRole() (owner/sales/warehouse/accountant);
			// "admin" is not a valid role and would break role-based UI filtering.
			Role:     "owner",
			Password: hashedStr,
		},
		{
			Email:    "admin@mail.com",
			Name:     "Admin Mail",
			Role:     "owner",
			Password: hashedStr,
		},
	}
	for _, u := range defaultUsers {
		if err := db.Create(&u).Error; err != nil {
			log.Printf("[WARN] Failed to seed user %s: %v", u.Email, err)
		}
	}
	log.Println("[INFO] Development seed complete: created default admin accounts (never run in production; override password via DEV_SEED_PASSWORD)")
}

// seedDefaultAccounts initializes standard Chart of Accounts if empty.
func seedDefaultAccounts(db *gorm.DB) {
	var count int64
	db.Model(&domainFinance.Account{}).Count(&count)
	if count > 0 {
		return
	}

	accounts := []domainFinance.Account{
		{Code: "1100", Name: "เงินสด (Cash)", Type: domainFinance.AccountTypeAsset, IsActive: true},
		{Code: "1110", Name: "เงินฝากธนาคาร (Bank)", Type: domainFinance.AccountTypeAsset, IsActive: true},
		{Code: "1200", Name: "ลูกหนี้การค้า (Accounts Receivable)", Type: domainFinance.AccountTypeAsset, IsActive: true},
		{Code: "1300", Name: "สินค้าคงเหลือ (Inventory)", Type: domainFinance.AccountTypeAsset, IsActive: true},
		{Code: "2000", Name: "เจ้าหนี้การค้า/GRNI (Accounts Payable / GRNI)", Type: domainFinance.AccountTypeLiability, IsActive: true},
		{Code: "2100", Name: "ภาษีขาย (VAT Output)", Type: domainFinance.AccountTypeLiability, IsActive: true},
		{Code: "3000", Name: "ทุน / ส่วนของเจ้าของ (Owner's Equity)", Type: domainFinance.AccountTypeEquity, IsActive: true},
		{Code: "4000", Name: "รายได้จากการขาย (Sales Revenue)", Type: domainFinance.AccountTypeRevenue, IsActive: true},
		{Code: "5000", Name: "ต้นทุนขาย (Cost of Goods Sold)", Type: domainFinance.AccountTypeExpense, IsActive: true},
		{Code: "5100", Name: "รับคืนและส่วนลดจ่าย (Sales Return)", Type: domainFinance.AccountTypeExpense, IsActive: true},
		{Code: "6000", Name: "ค่าใช้จ่ายในการดำเนินงาน (Operating Expense)", Type: domainFinance.AccountTypeExpense, IsActive: true},
	}

	for _, acc := range accounts {
		if err := db.Create(&acc).Error; err != nil {
			log.Printf("[WARN] Failed to seed account %s: %v", acc.Code, err)
		}
	}
	log.Println("[INFO] Chart of Accounts seed complete")
}
