package database

import (
	"fmt"
	"log"

	"chawy-erp-api/models"

	"golang.org/x/crypto/bcrypt"
)

// CleanMockData deletes all transactional and mock data from the database
func CleanMockData() {
	log.Println("Cleaning up mock data from database...")
	tables := []string{
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
		"audit_events",
		"sampling_recipients",
		"sampling_campaigns",
	}
	for _, t := range tables {
		if err := DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", t)).Error; err != nil {
			log.Printf("Warning: failed to truncate table %s: %v", t, err)
		}
	}
	log.Println("Mock data cleanup finished.")
}

// SeedData populates the database with initial configurations and admin user
func SeedData() {
	log.Println("Checking database to seed missing configuration/default data...")

	var userCount int64
	DB.Model(&models.AppUser{}).Count(&userCount)
	if userCount == 0 {
		log.Println("Seeding default Admin and Roles...")
		// Hash password "admin123"
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}
		hashedPasswordStr := string(hashedPassword)

		users := []models.AppUser{
			{ID: "USR-001", Name: "Chawy", Role: "owner", Password: hashedPasswordStr},
			{ID: "USR-002", Name: "จอย", Role: "sales", Password: hashedPasswordStr},
			{ID: "USR-003", Name: "แพร", Role: "warehouse", Password: hashedPasswordStr},
			{ID: "USR-004", Name: "จ็อบ", Role: "accountant", Password: hashedPasswordStr},
			{ID: "admin", Name: "Admin User", Role: "owner", Password: hashedPasswordStr},
		}
		for _, u := range users {
			DB.Create(&u)
		}
	}

	var settingsCount int64
	DB.Model(&models.CompanySettings{}).Count(&settingsCount)
	if settingsCount == 0 {
		log.Println("Seeding Settings...")
		company := models.CompanySettings{
			Name:          "Chawy Pet Food",
			TaxID:         "0123456789012",
			Address:       "123 ถ.สุขุมวิท แขวงคลองเตย เขตคลองเตย กรุงเทพฯ 10110",
			Phone:         "02-123-4567",
			Email:         "hello@chawypet.com",
			Website:       "www.chawypet.com",
			Currency:      "THB",
			VatRate:       7,
			InvoicePrefix: "INV-2026-",
			SoPrefix:      "SO-2026-",
		}
		DB.Create(&company)
	}

	var notifCount int64
	DB.Model(&models.NotificationSettings{}).Count(&notifCount)
	if notifCount == 0 {
		log.Println("Seeding Notification Settings...")
		notifications := models.NotificationSettings{
			NearExpiry:     true,
			NearExpiryDays: 30,
			LowStock:       true,
			LatePO:         true,
			NewSO:          true,
			PaymentDue:     true,
		}
		DB.Create(&notifications)
	}

	var moduleCount int64
	DB.Model(&models.ModuleSettings{}).Count(&moduleCount)
	if moduleCount == 0 {
		log.Println("Seeding Module Settings...")
		modules := models.ModuleSettings{
			Quotation:        true,
			SalesOrders:      true,
			Invoice:          true,
			Returns:          true,
			PurchaseReq:      true,
			PurchaseOrder:    true,
			SkuMaster:        true,
			StockBalance:     true,
			GoodsReceive:     true,
			GoodsIssue:       true,
			StockTransfer:    true,
			StockCheck:       true,
			Expenses:         true,
			PlReport:         true,
			Budget:           true,
			TiktokOrders:     true,
			LiveContent:      true,
			ManualOrder:      true,
			TiktokCalculator: true,
			Sampling:         true,
			UserManagement:   true,
			TiktokSetup:      true,
		}
		DB.Create(&modules)
	}

	var payrollCount int64
	DB.Model(&models.LivePayrollSettings{}).Count(&payrollCount)
	if payrollCount == 0 {
		log.Println("Seeding Live Payroll Settings...")
		payroll := models.LivePayrollSettings{
			HourlyRate: 120,
			ClipBonus:  100,
		}
		DB.Create(&payroll)
	}

	log.Println("Seeding configuration/default data completed.")
}
