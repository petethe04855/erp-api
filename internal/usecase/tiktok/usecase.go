package tiktok

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chawy-erp-api/config"
	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	pkgCrypto "chawy-erp-api/pkg/crypto"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
	pkgTikTok "chawy-erp-api/pkg/tiktok"

	"gorm.io/gorm"
)

type MapSKUInput struct {
	TikTokSKU string `json:"tiktok_sku"`
	LocalSKU  string `json:"local_sku"`
}

type SyncOrderItemInput struct {
	TikTokSKU string  `json:"tiktok_sku"`
	Quantity  int     `json:"quantity"`
	Qty       int     `json:"qty"` // alias accepted for compatibility
	Price     float64 `json:"price"`
}

type SyncOrderInput struct {
	TikTokOrderID string               `json:"tiktok_order_id"`
	CustomerName  string               `json:"customer_name"`
	Items         []SyncOrderItemInput `json:"items"`
}

type ConnectionResponse struct {
	Connected             bool      `json:"connected"`
	NeedsReauthorization  bool      `json:"needsReauthorization"`
	ShopCipher            string    `json:"shopCipher,omitempty"`
	SellerName            string    `json:"sellerName,omitempty"`
	SellerBaseRegion      string    `json:"sellerBaseRegion,omitempty"`
	GrantedScopes         string    `json:"grantedScopes,omitempty"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt,omitempty"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt,omitempty"`
}

type ConnectURLResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
}

type CallbackResponse struct {
	Connected  bool   `json:"connected"`
	Message    string `json:"message"`
	ShopCipher string `json:"shopCipher"`
	SellerName string `json:"sellerName"`
}

type SyncResultResponse struct {
	Synced                 int      `json:"synced"`
	Days                   int      `json:"days"`
	StockDeducted          int      `json:"stockDeducted"`
	StockDeductionErrors   []string `json:"stockDeductionErrors,omitempty"`
	StockDeductionWarnings []string `json:"stockDeductionWarnings,omitempty"`
}

type StockPreviewItem struct {
	TikTokSKU   string `json:"tiktokSku"`
	ERPSKU      string `json:"erpSku"`
	ProductName string `json:"productName"`
	TikTokStock int    `json:"tiktokStock"`
	ERPStock    int    `json:"erpStock"`
	Difference  int    `json:"difference"`
}

type Usecase interface {
	// Connection & OAuth
	GetConnection(ctx context.Context) (*ConnectionResponse, error)
	StartConnect(ctx context.Context) (*ConnectURLResponse, error)
	HandleCallback(ctx context.Context, code, state string) (*CallbackResponse, error)

	// Webhook
	ReceiveWebhook(ctx context.Context, eventID, eventType, timestamp, signature string, payload []byte) error

	// Orders & Sync
	SyncOrders(ctx context.Context, days int) (*SyncResultResponse, error)
	GetSyncRuns(ctx context.Context, limit int) ([]domainTikTok.TiktokSyncRun, error)
	GetOrders(ctx context.Context, limit int) ([]domainTikTok.TiktokOrder, error)
	ListOrders(ctx context.Context, query domainTikTok.OrderQuery) ([]domainTikTok.TiktokOrder, int64, error)

	// Products & Preview
	GetProducts(ctx context.Context, pageToken string, pageSize int) (*pkgTikTok.ProductSearchResponse, error)
	GetStockSyncPreview(ctx context.Context) ([]StockPreviewItem, error)

	// Mappings & Manual Sync
	SaveSKUMapping(ctx context.Context, in MapSKUInput) error
	ListSKUMappings(ctx context.Context) ([]domainTikTok.SKUMapping, error)
	SyncOrder(ctx context.Context, in SyncOrderInput) error
	GetSyncLogs(ctx context.Context, limit int) ([]domainTikTok.SyncLog, error)
}

type tiktokUsecase struct {
	cfg          *config.Config
	db           *gorm.DB
	tiktokRepo   domainTikTok.Repository
	tiktokClient *pkgTikTok.Client
	orderUsecase domainOrder.Repository
	formulaRepo  domainFormula.Repository
	skuRepo      domainSKU.Repository
	bundleRepo   domainBundle.Repository
	stockRepo    domainStock.Repository
}

func NewTikTokUsecase(
	cfg *config.Config,
	db *gorm.DB,
	tiktokRepo domainTikTok.Repository,
	orderRepo domainOrder.Repository,
	formulaRepo domainFormula.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	return &tiktokUsecase{
		cfg:          cfg,
		db:           db,
		tiktokRepo:   tiktokRepo,
		tiktokClient: pkgTikTok.NewClient(),
		orderUsecase: orderRepo,
		formulaRepo:  formulaRepo,
		skuRepo:      skuRepo,
		bundleRepo:   bundleRepo,
		stockRepo:    stockRepo,
	}
}

func (u *tiktokUsecase) GetConnection(ctx context.Context) (*ConnectionResponse, error) {
	conn, err := u.tiktokRepo.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.AccessToken == "" {
		return &ConnectionResponse{Connected: false}, nil
	}

	needsReauth := !conn.RefreshTokenExpiresAt.After(time.Now())
	return &ConnectionResponse{
		Connected:             true,
		NeedsReauthorization:  needsReauth,
		ShopCipher:            conn.ShopCipher,
		SellerName:            conn.SellerName,
		SellerBaseRegion:      conn.SellerBaseRegion,
		GrantedScopes:         conn.GrantedScopes,
		AccessTokenExpiresAt:  conn.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: conn.RefreshTokenExpiresAt,
	}, nil
}

func (u *tiktokUsecase) StartConnect(ctx context.Context) (*ConnectURLResponse, error) {
	if u.cfg.TikTokAppKey == "" || u.cfg.TikTokAppSecret == "" {
		return nil, appErrors.NewAppError("CONFIG_ERROR", "TikTok is not configured: TIKTOK_APP_KEY and TIKTOK_APP_SECRET are required", 503)
	}
	if u.cfg.TikTokServiceID == "" {
		return nil, appErrors.NewAppError("CONFIG_ERROR", "TikTok authorization is not configured: TIKTOK_SERVICE_ID is required", 503)
	}

	stateRaw := make([]byte, 32)
	if _, err := rand.Read(stateRaw); err != nil {
		return nil, fmt.Errorf("failed to generate random state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateRaw)
	stateHashBytes := sha256.Sum256([]byte(state))
	stateHash := hex.EncodeToString(stateHashBytes[:])

	oauthState := &domainTikTok.TiktokOAuthState{
		StateHash: stateHash,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := u.tiktokRepo.CreateOAuthState(ctx, oauthState); err != nil {
		return nil, fmt.Errorf("could not save OAuth state: %w", err)
	}

	authURL := u.cfg.TikTokAuthorizeURL
	if authURL == "" {
		authURL = "https://services.tiktokshop.com/open/authorize"
	}
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TIKTOK_AUTHORIZE_URL: %w", err)
	}

	q := parsedURL.Query()
	q.Set("service_id", u.cfg.TikTokServiceID)
	q.Set("state", state)
	parsedURL.RawQuery = q.Encode()

	return &ConnectURLResponse{AuthorizationURL: parsedURL.String()}, nil
}

func (u *tiktokUsecase) HandleCallback(ctx context.Context, code, state string) (*CallbackResponse, error) {
	if code == "" || state == "" {
		return nil, appErrors.NewAppError("BAD_REQUEST", "code and state parameters are required", 400)
	}
	if u.cfg.TikTokAppKey == "" || u.cfg.TikTokAppSecret == "" {
		return nil, appErrors.NewAppError("CONFIG_ERROR", "TikTok credentials are not configured", 503)
	}

	stateHashBytes := sha256.Sum256([]byte(state))
	stateHash := hex.EncodeToString(stateHashBytes[:])

	valid, err := u.tiktokRepo.ValidateAndConsumeOAuthState(ctx, stateHash)
	if err != nil || !valid {
		return nil, appErrors.NewAppError("INVALID_STATE", "Invalid or expired TikTok authorization response", 400)
	}

	tokenResp, err := u.tiktokClient.ExchangeCode(u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, code)
	if err != nil {
		return nil, appErrors.NewAppError("TIKTOK_API_ERROR", err.Error(), 502)
	}

	// Encrypt tokens before saving
	encKey := u.getEncryptionKey()
	encAccess, err := pkgCrypto.EncryptAESGCM(tokenResp.Data.AccessToken, encKey)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt access token: %w", err)
	}
	encRefresh, err := pkgCrypto.EncryptAESGCM(tokenResp.Data.RefreshToken, encKey)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt refresh token: %w", err)
	}

	conn := &domainTikTok.TiktokConnection{
		ID:                    1,
		AccessToken:           encAccess,
		RefreshToken:          encRefresh,
		AccessTokenExpiresAt:  time.Unix(tokenResp.Data.AccessTokenExpireIn, 0),
		RefreshTokenExpiresAt: time.Unix(tokenResp.Data.RefreshTokenExpireIn, 0),
		ShopCipher:            tokenResp.Data.ShopCipher,
		SellerName:            tokenResp.Data.SellerName,
		SellerBaseRegion:      tokenResp.Data.SellerBaseRegion,
		GrantedScopes:         strings.Join(tokenResp.Data.GrantedScopes, ","),
		UpdatedAt:             time.Now(),
	}

	// If shop cipher is missing in token response, fetch authorized shops
	if conn.ShopCipher == "" {
		shopsResp, err := u.tiktokClient.GetAuthorizedShops(u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, tokenResp.Data.AccessToken)
		if err == nil && shopsResp != nil && len(shopsResp.Data.Shops) > 0 {
			conn.ShopCipher = shopsResp.Data.Shops[0].Cipher
			if conn.SellerName == "" {
				conn.SellerName = shopsResp.Data.Shops[0].Name
			}
			if conn.SellerBaseRegion == "" {
				conn.SellerBaseRegion = shopsResp.Data.Shops[0].Region
			}
		}
	}

	if err := u.tiktokRepo.SaveConnection(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to save connection: %w", err)
	}

	return &CallbackResponse{
		Connected:  true,
		Message:    "เชื่อมต่อ TikTok Shop สำเร็จ",
		ShopCipher: conn.ShopCipher,
		SellerName: conn.SellerName,
	}, nil
}

func (u *tiktokUsecase) ReceiveWebhook(ctx context.Context, eventID, eventType, timestamp, signature string, payload []byte) error {
	secret := strings.TrimSpace(u.cfg.TikTokWebhookSecret)
	if secret == "" {
		return appErrors.NewAppError("CONFIG_ERROR", "TikTok webhook secret is not configured", 503)
	}

	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || math.Abs(float64(time.Now().Unix()-unixTimestamp)) > 300 {
		return appErrors.NewAppError("UNAUTHORIZED", "Invalid webhook timestamp", 401)
	}

	if err := pkgTikTok.VerifyWebhookSignature(eventID, timestamp, signature, string(payload), secret); err != nil {
		return appErrors.NewAppError("UNAUTHORIZED", err.Error(), 401)
	}

	alreadyExists, err := u.tiktokRepo.IsWebhookEventRecorded(ctx, eventID)
	if err != nil {
		return err
	}
	if alreadyExists {
		return nil // Idempotent success
	}

	event := &domainTikTok.TiktokWebhookEvent{
		EventID:    eventID,
		EventType:  eventType,
		Payload:    string(payload),
		ReceivedAt: time.Now().UTC(),
	}
	return u.tiktokRepo.RecordWebhookEvent(ctx, event)
}

func (u *tiktokUsecase) SyncOrders(ctx context.Context, days int) (*SyncResultResponse, error) {
	if days < 1 || days > 90 {
		days = 30
	}

	run := &domainTikTok.TiktokSyncRun{
		StartedAt: time.Now().UTC(),
		Status:    "Running",
		Days:      days,
	}
	_ = u.tiktokRepo.CreateSyncRun(ctx, run)

	conn, err := u.ensureAccessToken(ctx)
	if err != nil {
		finishErr := time.Now().UTC()
		run.FinishedAt = &finishErr
		run.Status = "Failed"
		run.Error = err.Error()
		_ = u.tiktokRepo.UpdateSyncRun(ctx, run)
		return nil, err
	}

	decryptedToken, err := pkgCrypto.DecryptAESGCM(conn.AccessToken, u.getEncryptionKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	now := time.Now()
	startTime := now.AddDate(0, 0, -days).Unix()
	endTime := now.Unix()

	var orders []domainTikTok.TiktokOrder
	pageToken := ""

	for page := 0; page < 25; page++ {
		resp, err := u.tiktokClient.SearchOrders(decryptedToken, conn.ShopCipher, u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, startTime, endTime, pageToken, 50)
		if err != nil {
			finishErr := time.Now().UTC()
			run.FinishedAt = &finishErr
			run.Status = "Failed"
			run.Error = err.Error()
			_ = u.tiktokRepo.UpdateSyncRun(ctx, run)
			return nil, err
		}

		for _, src := range resp.Data.Orders {
			items := make([]domainTikTok.TiktokOrderItem, 0, len(src.LineItems))
			totalQty := 0
			itemAmount := 0.0

			for _, line := range src.LineItems {
				qty := line.Quantity
				if qty < 1 {
					qty = 1
				}
				unitPrice, _ := strconv.ParseFloat(line.SalePrice, 64)
				amount := unitPrice * float64(qty)
				totalQty += qty
				itemAmount += amount

				items = append(items, domainTikTok.TiktokOrderItem{
					OrderID:     src.ID,
					LineItemID:  line.ID,
					ProductName: line.ProductName,
					SKU:         line.SellerSKU,
					Qty:         qty,
					UnitPrice:   unitPrice,
					Amount:      amount,
				})
			}

			amount, _ := strconv.ParseFloat(src.Payment.TotalAmount, 64)
			if amount == 0 {
				amount = itemAmount
			}

			product, sku := "", ""
			if len(items) > 0 {
				product, sku = items[0].ProductName, items[0].SKU
			}
			if len(items) > 1 {
				product = fmt.Sprintf("%s +%d รายการ", product, len(items)-1)
			}

			date := time.Unix(src.CreateTime, 0).In(time.FixedZone("Asia/Bangkok", 7*60*60)).Format("2006-01-02")
			orders = append(orders, domainTikTok.TiktokOrder{
				ID:      src.ID,
				Date:    date,
				Product: product,
				SKU:     sku,
				Qty:     totalQty,
				Amount:  amount,
				Status:  src.Status,
				Items:   items,
			})
		}

		pageToken = resp.Data.NextPageToken
		if pageToken == "" {
			break
		}
	}

	// Upsert Orders in DB
	if err := u.tiktokRepo.UpsertOrders(ctx, orders); err != nil {
		finishErr := time.Now().UTC()
		run.FinishedAt = &finishErr
		run.Status = "Failed"
		run.Error = err.Error()
		_ = u.tiktokRepo.UpdateSyncRun(ctx, run)
		return nil, err
	}

	// Process stock deduction for newly shipped/delivered orders
	// FULL-06/25: deductStockForOrder runs everything in ONE transaction
	// (stock, movements, processed flag) and returns an error when any line
	// cannot be deducted. Orders are never marked StockDeducted when lines
	// were skipped — they stay pending for retry after mapping fixes.
	deductedCount := 0
	var deductionErrors []string
	var deductionWarnings []string

	for _, order := range orders {
		if !tiktokOrderNeedsStockDeduction(order.Status) {
			continue
		}

		didDeduct, warnings, err := u.deductStockForOrder(ctx, order.ID)
		if err != nil {
			// Order stays StockDeducted=false so the next sync retries it.
			deductionErrors = append(deductionErrors, fmt.Sprintf("%s: %s", order.ID, err.Error()))
			continue
		}
		if didDeduct {
			deductedCount++
		}
		for _, w := range warnings {
			deductionWarnings = append(deductionWarnings, fmt.Sprintf("%s: %s", order.ID, w))
		}
	}

	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	run.Status = "Completed"
	run.Synced = len(orders)
	run.StockDeducted = deductedCount
	_ = u.tiktokRepo.UpdateSyncRun(ctx, run)

	return &SyncResultResponse{
		Synced:                 len(orders),
		Days:                   days,
		StockDeducted:          deductedCount,
		StockDeductionErrors:   deductionErrors,
		StockDeductionWarnings: deductionWarnings,
	}, nil
}

// deductStockForOrder deducts every line of a TikTok order inside ONE
// transaction covering the order-row lock, the stock_deducted re-check, stock,
// movements and the processed flag (FULL-06). The order row is locked with
// SELECT ... FOR UPDATE and the flag is re-checked inside the transaction so
// two concurrent sync workers cannot both read StockDeducted=false and
// double-deduct stock. Any missing mapping/SKU or insufficient stock fails the
// whole order and leaves StockDeducted=false so the next sync retries it
// (FULL-25).
func (u *tiktokUsecase) deductStockForOrder(ctx context.Context, orderID string) (bool, []string, error) {
	warehouseID := uint(1)
	didDeduct := false
	var warnings []string

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := database.WithTxContext(ctx, tx)

		// Lock the order row and re-check the flag inside the same transaction
		// that deducts stock — closes the concurrent-sync race.
		order, err := u.tiktokRepo.GetOrderByIDForUpdate(txCtx, orderID)
		if err != nil {
			return err
		}
		if order == nil || order.StockDeducted {
			return nil // already processed (or gone) — nothing to do
		}

		// Step 1: Collect and aggregate all physical SKU quantities to deduct across all items
		// (Resolving Inventory Formulas if defined; otherwise directly deducting the SKU)
		// Step 1: Map TikTok items and resolve components via shared DeductionResolver
		itemsToResolve := make([]usecaseStock.ItemToResolve, 0, len(order.Items))
		for _, item := range order.Items {
			if item.Qty <= 0 {
				continue
			}
			erpSKUCode := strings.ToUpper(strings.TrimSpace(item.SKU))
			mapping, err := u.tiktokRepo.GetMapping(txCtx, erpSKUCode)
			if err != nil {
				return fmt.Errorf("mapping lookup failed for %s: %w", item.SKU, err)
			}
			if mapping != nil && mapping.LocalSKU != "" {
				erpSKUCode = strings.ToUpper(strings.TrimSpace(mapping.LocalSKU))
			}
			itemsToResolve = append(itemsToResolve, usecaseStock.ItemToResolve{
				SKU:      erpSKUCode,
				Quantity: item.Qty,
			})
		}

		resolver := usecaseStock.NewDeductionResolver(u.db, u.formulaRepo, u.skuRepo)
		resolvedItems, err := resolver.ResolveDeductionItems(txCtx, itemsToResolve, "TIKTOK", order.ID)
		if err != nil {
			return err
		}

		// Aggregate resolved items by SKUCode
		type AggregatedItem struct {
			SKUCode           string
			Quantity          int
			SourceFormulaCode string
			RefType           string
			Note              string
		}
		aggMap := make(map[string]*AggregatedItem)
		for _, ri := range resolvedItems {
			if entry, exists := aggMap[ri.SKUCode]; exists {
				entry.Quantity += ri.Quantity
			} else {
				aggMap[ri.SKUCode] = &AggregatedItem{
					SKUCode:           ri.SKUCode,
					Quantity:          ri.Quantity,
					SourceFormulaCode: ri.SourceFormulaCode,
					RefType:           ri.RefType,
					Note:              ri.Note,
				}
			}
		}

		// Step 2: Validate all SKUs exist and have sufficient stock under FOR UPDATE locks
		type ValidatedStock struct {
			Target *AggregatedItem
			SKU    *domainSKU.SKU
			Stock  *domainStock.Stock
			Lots   []domainStock.StockLot
		}
		var toDeduct []ValidatedStock

		for _, target := range aggMap {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, target.SKUCode)
			if err != nil {
				return err
			}
			if skuEntity == nil {
				return appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %s ไม่พบในระบบ", target.SKUCode), 404)
			}

			stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
			if err != nil {
				return err
			}
			avail := 0
			if stk != nil {
				avail = stk.AvailableQty
			}
			if stk == nil || avail < target.Quantity {
				return appErrors.NewAppError(
					"INSUFFICIENT_STOCK",
					fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, พร้อมขาย %d (Order %s)", target.SKUCode, target.Quantity, avail, order.ID),
					409,
				)
			}

			lots, err := u.stockRepo.GetAvailableLotsForUpdate(txCtx, skuEntity.ID, warehouseID)
			if err != nil {
				return err
			}

			toDeduct = append(toDeduct, ValidatedStock{
				Target: target,
				SKU:    skuEntity,
				Stock:  stk,
				Lots:   lots,
			})
		}

		// Step 3: Perform FEFO lot deductions, stock updates, and write movements with metadata
		for _, vs := range toDeduct {
			updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, vs.SKU.ID, warehouseID, -vs.Target.Quantity)
			if err != nil {
				return err
			}

			needed := vs.Target.Quantity
			currentStockQty := vs.Stock.Quantity
			for _, lot := range vs.Lots {
				if needed <= 0 {
					break
				}
				lotAvail := lot.Quantity - lot.ReservedQty
				if lotAvail <= 0 {
					continue
				}
				deductQty := lotAvail
				if deductQty > needed {
					deductQty = needed
				}

				if _, err := u.stockRepo.DeductLotQuantity(txCtx, lot.ID, deductQty); err != nil {
					return err
				}

				movement := &domainStock.StockMovement{
					SKUID:             vs.SKU.ID,
					SKUCode:           vs.SKU.SKU,
					WarehouseID:       warehouseID,
					StockLotID:        &lot.ID,
					SourceFormulaCode: vs.Target.SourceFormulaCode,
					Channel:           "TIKTOK",
					Type:              domainStock.MovementOut,
					Quantity:          deductQty,
					BeforeQty:         currentStockQty,
					AfterQty:          currentStockQty - deductQty,
					ReferenceType:     vs.Target.RefType,
					ReferenceID:       order.ID,
					Note:              fmt.Sprintf("%s (Lot: %s)", vs.Target.Note, lot.LotNumber),
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}
				currentStockQty -= deductQty
				needed -= deductQty
			}

			// If remaining needed > 0 (e.g. stock without lot records), write remaining movement
			if needed > 0 {
				movement := &domainStock.StockMovement{
					SKUID:             vs.SKU.ID,
					SKUCode:           vs.SKU.SKU,
					WarehouseID:       warehouseID,
					SourceFormulaCode: vs.Target.SourceFormulaCode,
					Channel:           "TIKTOK",
					Type:              domainStock.MovementOut,
					Quantity:          needed,
					BeforeQty:         currentStockQty,
					AfterQty:          updatedStk.Quantity,
					ReferenceType:     vs.Target.RefType,
					ReferenceID:       order.ID,
					Note:              vs.Target.Note,
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}
			}
		}

		// Mark the order processed inside the same transaction so a rollback
		// also rolls back the flag (FULL-06).
		if err := u.tiktokRepo.UpdateOrderStockDeducted(txCtx, order.ID, true); err != nil {
			return err
		}
		didDeduct = true
		return nil
	})

	if err != nil {
		return false, warnings, err
	}
	return didDeduct, warnings, nil
}

func (u *tiktokUsecase) GetSyncRuns(ctx context.Context, limit int) ([]domainTikTok.TiktokSyncRun, error) {
	return u.tiktokRepo.ListSyncRuns(ctx, limit)
}

func (u *tiktokUsecase) GetOrders(ctx context.Context, limit int) ([]domainTikTok.TiktokOrder, error) {
	return u.tiktokRepo.ListRecentOrders(ctx, limit)
}

func (u *tiktokUsecase) ListOrders(ctx context.Context, query domainTikTok.OrderQuery) ([]domainTikTok.TiktokOrder, int64, error) {
	return u.tiktokRepo.ListOrders(ctx, query)
}

func (u *tiktokUsecase) GetProducts(ctx context.Context, pageToken string, pageSize int) (*pkgTikTok.ProductSearchResponse, error) {
	conn, err := u.ensureAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	decryptedToken, err := pkgCrypto.DecryptAESGCM(conn.AccessToken, u.getEncryptionKey())
	if err != nil {
		return nil, err
	}

	return u.tiktokClient.SearchProducts(decryptedToken, conn.ShopCipher, u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, pageToken, pageSize)
}

func (u *tiktokUsecase) GetStockSyncPreview(ctx context.Context) ([]StockPreviewItem, error) {
	conn, err := u.ensureAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	decryptedToken, err := pkgCrypto.DecryptAESGCM(conn.AccessToken, u.getEncryptionKey())
	if err != nil {
		return nil, err
	}

	resp, err := u.tiktokClient.SearchProducts(decryptedToken, conn.ShopCipher, u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, "", 50)
	if err != nil {
		return nil, err
	}

	var preview []StockPreviewItem
	warehouseID := uint(1)

	for _, p := range resp.Data.Products {
		for _, sku := range p.SKUs {
			tiktokStock := 0
			if len(sku.StockInfos) > 0 {
				tiktokStock = sku.StockInfos[0].AvailableStock
			}

			erpSKUCode := sku.SellerSKU
			mapping, _ := u.tiktokRepo.GetMapping(ctx, sku.SellerSKU)
			if mapping != nil && mapping.LocalSKU != "" {
				erpSKUCode = mapping.LocalSKU
			}

			erpStock := 0
			skuEntity, _ := u.skuRepo.FindBySKU(ctx, erpSKUCode)
			if skuEntity != nil {
				stk, _ := u.stockRepo.GetBySKUID(ctx, skuEntity.ID, warehouseID)
				if stk != nil {
					erpStock = stk.AvailableQty
				}
			}

			preview = append(preview, StockPreviewItem{
				TikTokSKU:   sku.SellerSKU,
				ERPSKU:      erpSKUCode,
				ProductName: p.Title,
				TikTokStock: tiktokStock,
				ERPStock:    erpStock,
				Difference:  erpStock - tiktokStock,
			})
		}
	}

	return preview, nil
}

func (u *tiktokUsecase) SaveSKUMapping(ctx context.Context, in MapSKUInput) error {
	m := &domainTikTok.SKUMapping{
		TikTokSKU: strings.ToUpper(strings.TrimSpace(in.TikTokSKU)),
		LocalSKU:  strings.ToUpper(strings.TrimSpace(in.LocalSKU)),
	}
	return u.tiktokRepo.SaveMapping(ctx, m)
}

func (u *tiktokUsecase) ListSKUMappings(ctx context.Context) ([]domainTikTok.SKUMapping, error) {
	return u.tiktokRepo.ListMappings(ctx)
}

// SyncOrder ingests a manually pushed TikTok order (FULL-26): persists the
// order and its items idempotently, then logs honestly.
func (u *tiktokUsecase) SyncOrder(ctx context.Context, in SyncOrderInput) error {
	if strings.TrimSpace(in.TikTokOrderID) == "" {
		return appErrors.NewAppError("INVALID_INPUT", "TikTok order ID is required", 400)
	}
	if len(in.Items) == 0 {
		return appErrors.NewAppError("INVALID_INPUT", "Order must contain at least one item", 400)
	}
	for _, it := range in.Items {
		if strings.TrimSpace(it.TikTokSKU) == "" || (it.Qty <= 0 && it.Quantity <= 0) {
			return appErrors.NewAppError("INVALID_INPUT", "Each item needs a SKU and a positive quantity", 400)
		}
	}

	// Idempotency: already-ingested orders are acknowledged without rewriting.
	existing, err := u.tiktokRepo.GetOrderByID(ctx, in.TikTokOrderID)
	if err != nil {
		return err
	}
	if existing != nil {
		logErr := u.tiktokRepo.CreateSyncLog(ctx, &domainTikTok.SyncLog{
			OrderNo: in.TikTokOrderID,
			Status:  "SKIPPED",
			Message: "Manual sync ignored: order already ingested",
		})
		if logErr != nil {
			return fmt.Errorf("order already ingested but sync log write failed: %w", logErr)
		}
		return nil
	}

	var total float64
	items := make([]domainTikTok.TiktokOrderItem, 0, len(in.Items))
	for _, it := range in.Items {
		qty := it.Qty
		if qty <= 0 {
			qty = it.Quantity
		}
		lineTotal := it.Price * float64(qty)
		total += lineTotal
		items = append(items, domainTikTok.TiktokOrderItem{
			SKU:       strings.ToUpper(strings.TrimSpace(it.TikTokSKU)),
			Qty:       qty,
			UnitPrice: it.Price,
			Amount:    lineTotal,
		})
	}

	customerName := strings.TrimSpace(in.CustomerName)
	if customerName == "" {
		customerName = "TikTok Customer"
	}

	order := &domainTikTok.TiktokOrder{
		ID:      in.TikTokOrderID,
		Date:    time.Now().Format("2006-01-02 15:04:05"),
		Product: items[0].SKU,
		SKU:     items[0].SKU,
		Qty: func() int {
			q := 0
			for _, it := range items {
				q += it.Qty
			}
			return q
		}(),
		Amount:   total,
		Status:   "SHIPPED",
		Imported: true,
		Items:    items,
	}
	_ = customerName // customer name is not persisted on the TikTok order header schema today

	// UpsertOrders writes header + items in one repository transaction.
	if err := u.tiktokRepo.UpsertOrders(ctx, []domainTikTok.TiktokOrder{*order}); err != nil {
		logStatus := "FAILED"
		_ = u.tiktokRepo.CreateSyncLog(ctx, &domainTikTok.SyncLog{
			OrderNo: in.TikTokOrderID,
			Status:  logStatus,
			Message: "Manual sync ingestion failed: " + err.Error(),
		})
		return fmt.Errorf("failed to ingest manual TikTok order %s: %w", in.TikTokOrderID, err)
	}

	if err := u.tiktokRepo.CreateSyncLog(ctx, &domainTikTok.SyncLog{
		OrderNo: in.TikTokOrderID,
		Status:  "SUCCESS",
		Message: fmt.Sprintf("Manual sync ingested order %s for %s with %d item line(s)", in.TikTokOrderID, customerName, len(items)),
	}); err != nil {
		// Order persisted but log write failed — surface it.
		return fmt.Errorf("order %s ingested but sync log write failed: %w", in.TikTokOrderID, err)
	}

	return nil
}

func (u *tiktokUsecase) GetSyncLogs(ctx context.Context, limit int) ([]domainTikTok.SyncLog, error) {
	return u.tiktokRepo.GetSyncLogs(ctx, limit)
}

// Helpers
func (u *tiktokUsecase) ensureAccessToken(ctx context.Context) (*domainTikTok.TiktokConnection, error) {
	conn, err := u.tiktokRepo.GetConnection(ctx)
	if err != nil || conn == nil || conn.AccessToken == "" {
		return nil, appErrors.NewAppError("UNAUTHORIZED", "TikTok Shop is not connected", 401)
	}

	encKey := u.getEncryptionKey()
	// If within 24-hour expiration window, refresh token
	if time.Until(conn.AccessTokenExpiresAt) <= pkgTikTok.TokenExpireGrace {
		if !conn.RefreshTokenExpiresAt.After(time.Now()) {
			return nil, appErrors.NewAppError("TOKEN_EXPIRED", "TikTok authorization has expired; reconnect the shop", 401)
		}

		decryptedRefresh, err := pkgCrypto.DecryptAESGCM(conn.RefreshToken, encKey)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt refresh token: %w", err)
		}

		refreshResp, err := u.tiktokClient.RefreshToken(u.cfg.TikTokAppKey, u.cfg.TikTokAppSecret, decryptedRefresh)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh tiktok token: %w", err)
		}

		newEncAccess, err := pkgCrypto.EncryptAESGCM(refreshResp.Data.AccessToken, encKey)
		if err != nil {
			return nil, err
		}
		conn.AccessToken = newEncAccess
		conn.AccessTokenExpiresAt = time.Unix(refreshResp.Data.AccessTokenExpireIn, 0)

		if refreshResp.Data.RefreshToken != "" {
			newEncRefresh, err := pkgCrypto.EncryptAESGCM(refreshResp.Data.RefreshToken, encKey)
			if err == nil {
				conn.RefreshToken = newEncRefresh
			}
		}
		if refreshResp.Data.RefreshTokenExpireIn > 0 {
			conn.RefreshTokenExpiresAt = time.Unix(refreshResp.Data.RefreshTokenExpireIn, 0)
		}

		conn.UpdatedAt = time.Now()
		// FULL-29: a refresh that is not persisted must not be reported as
		// success — the next call would read the stale token again.
		if err := u.tiktokRepo.SaveConnection(ctx, conn); err != nil {
			return nil, fmt.Errorf("refreshed tiktok token but failed to persist it: %w", err)
		}
	}

	return conn, nil
}

func (u *tiktokUsecase) getEncryptionKey() string {
	if u.cfg.TikTokTokenEncryptionKey != "" {
		return u.cfg.TikTokTokenEncryptionKey
	}
	// Fallback to a derived 32-byte key from JWT secret if encryption key is not set
	hash := sha256.Sum256([]byte(u.cfg.JWTSecret + "_tiktok_salt_32b"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func tiktokOrderNeedsStockDeduction(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SHIPPED", "IN_TRANSIT", "DELIVERED", "COMPLETED":
		return true
	default:
		return false
	}
}
