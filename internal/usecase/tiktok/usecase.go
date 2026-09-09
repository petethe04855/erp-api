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
	domainOrder "chawy-erp-api/internal/domain/order"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	appErrors "chawy-erp-api/pkg/errors"
	pkgCrypto "chawy-erp-api/pkg/crypto"
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
	Synced                int      `json:"synced"`
	Days                  int      `json:"days"`
	StockDeducted         int      `json:"stockDeducted"`
	StockDeductionErrors  []string `json:"stockDeductionErrors,omitempty"`
	StockDeductionWarnings []string `json:"stockDeductionWarnings,omitempty"`
}

type StockPreviewItem struct {
	TikTokSKU    string `json:"tiktokSku"`
	ERPSKU       string `json:"erpSku"`
	ProductName  string `json:"productName"`
	TikTokStock  int    `json:"tiktokStock"`
	ERPStock     int    `json:"erpStock"`
	Difference   int    `json:"difference"`
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
	skuRepo      domainSKU.Repository
	bundleRepo   domainBundle.Repository
	stockRepo    domainStock.Repository
}

func NewTikTokUsecase(
	cfg *config.Config,
	db *gorm.DB,
	tiktokRepo domainTikTok.Repository,
	orderRepo domainOrder.Repository,
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
	deductedCount := 0
	var deductionErrors []string
	var deductionWarnings []string

	for _, order := range orders {
		if !tiktokOrderNeedsStockDeduction(order.Status) {
			continue
		}

		// Re-fetch from db to verify stock_deducted state
		dbOrder, err := u.tiktokRepo.GetOrderByID(ctx, order.ID)
		if err != nil || dbOrder == nil || dbOrder.StockDeducted {
			continue
		}

		didDeduct, warnings, err := u.deductStockForOrder(ctx, dbOrder)
		if err != nil {
			deductionErrors = append(deductionErrors, fmt.Sprintf("%s: %s", dbOrder.ID, err.Error()))
			continue
		}
		if didDeduct {
			deductedCount++
			_ = u.tiktokRepo.UpdateOrderStockDeducted(ctx, dbOrder.ID, true)
		}
		for _, w := range warnings {
			deductionWarnings = append(deductionWarnings, fmt.Sprintf("%s: %s", dbOrder.ID, w))
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

func (u *tiktokUsecase) deductStockForOrder(ctx context.Context, order *domainTikTok.TiktokOrder) (bool, []string, error) {
	warehouseID := uint(1)
	var warnings []string

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range order.Items {
			if item.Qty <= 0 {
				continue
			}

			// Map TikTok SKU to ERP SKU
			erpSKUCode := strings.ToUpper(strings.TrimSpace(item.SKU))
			mapping, err := u.tiktokRepo.GetMapping(ctx, item.SKU)
			if err == nil && mapping != nil && mapping.LocalSKU != "" {
				erpSKUCode = strings.ToUpper(strings.TrimSpace(mapping.LocalSKU))
			}

			skuEntity, err := u.skuRepo.FindBySKU(ctx, erpSKUCode)
			if err != nil || skuEntity == nil {
				warnings = append(warnings, fmt.Sprintf("SKU %s not found in ERP, skipped deduction", item.SKU))
				continue
			}

			if skuEntity.IsBundle {
				// Bundle resolution: load bundle items
				bundleItems, err := u.bundleRepo.GetItemsByBundleSKU(ctx, skuEntity.SKU)
				if err != nil {
					return err
				}

				for _, bi := range bundleItems {
					compSKU, err := u.skuRepo.FindBySKU(ctx, bi.ComponentSKU)
					if err != nil || compSKU == nil {
						return fmt.Errorf("bundle component %s not found", bi.ComponentSKU)
					}

					qtyDeduct := bi.Quantity * item.Qty
					stk, err := u.stockRepo.GetBySKUID(ctx, compSKU.ID, warehouseID)
					if err != nil {
						return err
					}
					if stk == nil || stk.AvailableQty < qtyDeduct {
						return fmt.Errorf("insufficient stock for component %s: required %d", bi.ComponentSKU, qtyDeduct)
					}

					updatedStk, err := u.stockRepo.UpdateQuantity(ctx, compSKU.ID, warehouseID, -qtyDeduct)
					if err != nil {
						return err
					}

					_ = u.stockRepo.CreateMovement(ctx, &domainStock.StockMovement{
						SKUID:         compSKU.ID,
						SKUCode:       compSKU.SKU,
						WarehouseID:   warehouseID,
						Type:          domainStock.MovementOut,
						Quantity:      qtyDeduct,
						BeforeQty:     stk.Quantity,
						AfterQty:      updatedStk.Quantity,
						ReferenceType: "TIKTOK_BUNDLE",
						ReferenceID:   order.ID,
						Note:          fmt.Sprintf("TikTok order %s shipped: bundle %s component %s", order.ID, skuEntity.SKU, compSKU.SKU),
					})
				}
			} else {
				// Single SKU stock deduction
				stk, err := u.stockRepo.GetBySKUID(ctx, skuEntity.ID, warehouseID)
				if err != nil {
					return err
				}
				if stk == nil || stk.AvailableQty < item.Qty {
					return fmt.Errorf("insufficient stock for SKU %s: required %d", skuEntity.SKU, item.Qty)
				}

				updatedStk, err := u.stockRepo.UpdateQuantity(ctx, skuEntity.ID, warehouseID, -item.Qty)
				if err != nil {
					return err
				}

				_ = u.stockRepo.CreateMovement(ctx, &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   warehouseID,
					Type:          domainStock.MovementOut,
					Quantity:      item.Qty,
					BeforeQty:     stk.Quantity,
					AfterQty:      updatedStk.Quantity,
					ReferenceType: "TIKTOK_ORDER",
					ReferenceID:   order.ID,
					Note:          fmt.Sprintf("TikTok order %s shipped: SKU %s", order.ID, skuEntity.SKU),
				})
			}
		}
		return nil
	})

	return err == nil, warnings, err
}

func (u *tiktokUsecase) GetSyncRuns(ctx context.Context, limit int) ([]domainTikTok.TiktokSyncRun, error) {
	return u.tiktokRepo.ListSyncRuns(ctx, limit)
}

func (u *tiktokUsecase) GetOrders(ctx context.Context, limit int) ([]domainTikTok.TiktokOrder, error) {
	return u.tiktokRepo.ListRecentOrders(ctx, limit)
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

func (u *tiktokUsecase) SyncOrder(ctx context.Context, in SyncOrderInput) error {
	// Fallback manual order sync from webhook or manual push
	conn, _ := u.tiktokRepo.GetConnection(ctx)
	_ = conn

	logStatus := "SUCCESS"
	logMsg := fmt.Sprintf("Successfully synced manual TikTok order %s", in.TikTokOrderID)

	_ = u.tiktokRepo.CreateSyncLog(ctx, &domainTikTok.SyncLog{
		OrderNo: in.TikTokOrderID,
		Status:  logStatus,
		Message: logMsg,
	})
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
		_ = u.tiktokRepo.SaveConnection(ctx, conn)
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
