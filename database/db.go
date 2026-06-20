package database

import (
	"fmt"
	"log"
	"os"

	"chawy-erp-api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// ConnectDB establishes a connection pool to PostgreSQL using DSN from env and runs AutoMigrate
func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")

		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "5432"
		}
		if user == "" {
			user = "postgres"
		}
		if password == "" {
			password = "password"
		}
		if dbname == "" {
			dbname = "chawy_erp"
		}

		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Bangkok",
			host, user, password, dbname, port)
	}

	log.Printf("Connecting to database using DSN configuration...")

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connection established successfully")

	if os.Getenv("CLEAN_DB") == "true" {
		log.Println("CLEAN_DB is true: Dropping all tables to ensure clean migration...")
		tables := []string{
			"audit_events",
			"sales_order_lines",
			"sales_orders",
			"invoices",
			"quotation_lines",
			"quotations",
			"expenses",
			"content_schedule_items",
			"live_sessions",
			"manual_orders",
			"month_budgets",
			"tiktok_orders",
			"bundle_components",
			"products",
			"goods_receive_items",
			"goods_receives",
			"purchase_order_items",
			"purchase_orders",
			"purchase_request_items",
			"purchase_requests",
			"stock_lots",
			"stock_movements",
			"stock_adjustment_items",
			"stock_adjustments",
			"stock_transfers",
			"goods_issues",
			"stock_returns",
			"sampling_recipients",
			"sampling_campaigns",
			"app_users",
			"company_settings",
			"notification_settings",
			"module_settings",
			"live_payroll_settings",
		}
		for _, t := range tables {
			DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
		}
		log.Println("All tables dropped successfully.")
	}

	// Drop incorrect polymorphic foreign key constraints if they exist before migrating
	log.Println("Pre-dropping incorrect foreign key constraints on polymorphic audit_events table...")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_sales_orders_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_invoices_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_purchase_orders_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_goods_receives_audit_trail")

	// Run AutoMigrate
	err = DB.AutoMigrate(
		&models.AppUser{},
		&models.CompanySettings{},
		&models.NotificationSettings{},
		&models.ModuleSettings{},
		&models.LivePayrollSettings{},
		&models.Product{},
		&models.BundleComponent{},
		&models.StockLot{},
		&models.GoodsIssue{},
		&models.StockReturn{},
		&models.StockAdjustment{},
		&models.StockTransfer{},
		&models.Quotation{},
		&models.SalesOrder{},
		&models.Invoice{},
		&models.PurchaseRequest{},
		&models.PurchaseOrder{},
		&models.GoodsReceive{},
		&models.StockMovement{},
		&models.Expense{},
		&models.MonthBudget{},
		&models.TiktokOrder{},
		&models.ManualOrder{},
		&models.ContentScheduleItem{},
		&models.LiveSession{},
		&models.SamplingCampaign{},

		// Dependent tables with foreign keys
		&models.StockAdjustmentItem{},
		&models.QuotationLine{},
		&models.SalesOrderLine{},
		&models.PurchaseRequestItem{},
		&models.PurchaseOrderItem{},
		&models.GoodsReceiveItem{},
		&models.SamplingRecipient{},
		&models.AuditEvent{},
	)
	if err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	// Drop incorrect polymorphic foreign key constraints if they exist
	log.Println("Dropping incorrect foreign key constraints on polymorphic audit_events table...")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_sales_orders_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_invoices_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_purchase_orders_audit_trail")
	DB.Exec("ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS fk_goods_receives_audit_trail")

	log.Println("Database schema migrated successfully")
}
