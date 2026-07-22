package database

import (
	"fmt"
	"log"
	"strings"

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
		"boms",
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
			if u.Email == "" {
				u.Email = strings.ToLower(u.ID) + "@chawypet.local"
			}
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

	var productCount int64
	DB.Model(&models.Product{}).Count(&productCount)
	if productCount == 0 {
		log.Println("Seeding Item Master Records for BOM demo...")
		products := []models.Product{
			{SKU: "FG-COOKIE", Name: "คุกกี้เนยสด 12 ชิ้น", Type: "Finished Product", Cost: 0, Price: 180, RetailPrice: 180, WholesalePrice: 150, BaseUnit: "box", IsActive: true},
			{SKU: "FG-BROWNIE", Name: "บราวนี่อัลมอนด์", Type: "Finished Product", Cost: 0, Price: 220, RetailPrice: 220, WholesalePrice: 190, BaseUnit: "box", IsActive: true},
			{SKU: "FG-CHICKEN-200G", Name: "อกไก่ปรุงสุก 200 กรัม", Type: "Finished Product", Cost: 0, Price: 89, RetailPrice: 89, WholesalePrice: 72, BaseUnit: "bag", IsActive: true},
			{SKU: "SC-MARINADE", Name: "ซอสหมักกลาง", Type: "Sub-component", Cost: 0, BaseUnit: "kg", IsBundle: true, IsActive: true},
			{SKU: "RM-CHICKEN", Name: "เนื้ออกไก่", Type: "Raw Material", Cost: 80, Stock: 100, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-FLOUR", Name: "แป้งสาลี", Type: "Raw Material", Cost: 42, Stock: 120, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-BUTTER", Name: "เนยสด", Type: "Raw Material", Cost: 210, Stock: 45, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-SUGAR", Name: "น้ำตาล", Type: "Raw Material", Cost: 28, Stock: 80, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-COCOA", Name: "ผงโกโก้", Type: "Raw Material", Cost: 180, Stock: 25, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-SOY", Name: "ซีอิ๊ว", Type: "Raw Material", Cost: 38, Stock: 40, BaseUnit: "kg", IsActive: true},
			{SKU: "RM-SPICE", Name: "เครื่องเทศ", Type: "Raw Material", Cost: 320, Stock: 10, BaseUnit: "kg", IsActive: true},
			{SKU: "PK-BAG-200G", Name: "ถุงบรรจุ 200 กรัม", Type: "Packaging", Cost: 1.2, Stock: 2000, BaseUnit: "piece", IsActive: true},
			{SKU: "PK-LABEL", Name: "ฉลากสินค้า", Type: "Packaging", Cost: 0.45, Stock: 5000, BaseUnit: "piece", IsActive: true},
			{SKU: "PK-BOX", Name: "กล่องสินค้า", Type: "Packaging", Cost: 6.5, Stock: 600, BaseUnit: "piece", IsActive: true},
		}
		for _, p := range products {
			DB.Create(&p)
		}
	}

	var componentCount int64
	DB.Model(&models.BundleComponent{}).Count(&componentCount)
	if componentCount == 0 {
		log.Println("Seeding BOM component mappings...")
		components := []models.BundleComponent{
			{BundleSku: "FG-COOKIE", ComponentSku: "RM-FLOUR", ComponentName: "แป้งสาลี", Qty: 12, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "FG-COOKIE", ComponentSku: "RM-BUTTER", ComponentName: "เนยสด", Qty: 8, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "FG-COOKIE", ComponentSku: "PK-BOX", ComponentName: "กล่องสินค้า", Qty: 100, Unit: "piece", ComponentType: "packaging", YieldFactor: 1},
			{BundleSku: "FG-BROWNIE", ComponentSku: "RM-FLOUR", ComponentName: "แป้งสาลี", Qty: 8, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "FG-BROWNIE", ComponentSku: "RM-COCOA", ComponentName: "ผงโกโก้", Qty: 5, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "FG-BROWNIE", ComponentSku: "PK-BOX", ComponentName: "กล่องสินค้า", Qty: 80, Unit: "piece", ComponentType: "packaging", YieldFactor: 1},
			{BundleSku: "SC-MARINADE", ComponentSku: "RM-SOY", ComponentName: "ซีอิ๊ว", Qty: 5, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "SC-MARINADE", ComponentSku: "RM-SUGAR", ComponentName: "น้ำตาล", Qty: 2, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "SC-MARINADE", ComponentSku: "RM-SPICE", ComponentName: "เครื่องเทศ", Qty: 0.8, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "SC-MARINADE", ComponentName: "น้ำ", Qty: 2.2, Unit: "kg", ComponentType: "expense", UnitCostOverride: 0, YieldFactor: 1},
			{BundleSku: "FG-CHICKEN-200G", ComponentSku: "RM-CHICKEN", ComponentName: "เนื้ออกไก่", Qty: 20, Unit: "kg", ComponentType: "material", YieldFactor: 1},
			{BundleSku: "FG-CHICKEN-200G", ComponentSku: "SC-MARINADE", ComponentName: "ซอสหมักกลาง", Qty: 3, Unit: "kg", ComponentType: "subcomponent", YieldFactor: 1},
			{BundleSku: "FG-CHICKEN-200G", ComponentSku: "PK-BAG-200G", ComponentName: "ถุงบรรจุ 200 กรัม", Qty: 100, Unit: "piece", ComponentType: "packaging", YieldFactor: 1},
			{BundleSku: "FG-CHICKEN-200G", ComponentSku: "PK-LABEL", ComponentName: "ฉลากสินค้า", Qty: 100, Unit: "piece", ComponentType: "packaging", YieldFactor: 1},
		}
		for _, c := range components {
			DB.Create(&c)
		}
	}

	var bomCount int64
	DB.Model(&models.BOM{}).Count(&bomCount)
	if bomCount == 0 {
		log.Println("Seeding Demo BOM Records...")
		demoBOMs := []models.BOM{
			{
				Code:           "BOM-COOKIE-V3",
				Name:           "คุกกี้เนยสด 12 ชิ้น",
				Version:        3,
				Status:         "Active",
				Kind:           "finished",
				Waste:          3.0,
				FgSku:          "FG-COOKIE",
				OutputQty:      100,
				OutputUnit:     "กล่อง",
				EffectiveDate:  "2026-07-19",
				ComponentCount: 3,
			},
			{
				Code:           "BOM-BROWNIE-V2",
				Name:           "บราวนี่อัลมอนด์",
				Version:        2,
				Status:         "Active",
				Kind:           "finished",
				Waste:          2.0,
				FgSku:          "FG-BROWNIE",
				OutputQty:      80,
				OutputUnit:     "กล่อง",
				EffectiveDate:  "2026-07-19",
				ComponentCount: 3,
			},
			{
				Code:           "BOM-MARINADE-V1",
				Name:           "ซอสหมักกลาง",
				Version:        1,
				Status:         "Active",
				Kind:           "subcomponent",
				Waste:          0.0,
				FgSku:          "SC-MARINADE",
				OutputQty:      10,
				OutputUnit:     "kg",
				EffectiveDate:  "2026-07-19",
				ComponentCount: 4,
			},
			{
				Code:           "BOM-CHICKEN-200G-V2",
				Name:           "อกไก่ปรุงสุก 200 กรัม",
				Version:        2,
				Status:         "Active",
				Kind:           "finished",
				Waste:          3.0,
				FgSku:          "FG-CHICKEN-200G",
				OutputQty:      100,
				OutputUnit:     "ถุง",
				EffectiveDate:  "2026-07-19",
				ComponentCount: 4,
			},
		}
		for _, b := range demoBOMs {
			DB.Create(&b)
		}
	}

	log.Println("Seeding configuration/default data completed.")
}
