package handler

import (
	"os"
	"strconv"
	"strings"

	usecaseTikTok "chawy-erp-api/internal/usecase/tiktok"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type TikTokHandler struct {
	usecase usecaseTikTok.Usecase
}

func NewTikTokHandler(usecase usecaseTikTok.Usecase) *TikTokHandler {
	return &TikTokHandler{usecase: usecase}
}

// GetConnection returns current connection status
func (h *TikTokHandler) GetConnection(c *fiber.Ctx) error {
	res, err := h.usecase.GetConnection(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

// StartConnect returns the OAuth authorization URL
func (h *TikTokHandler) StartConnect(c *fiber.Ctx) error {
	res, err := h.usecase.StartConnect(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

// Callback handles the OAuth redirect with authorization code and state
func (h *TikTokHandler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	_, err := h.usecase.HandleCallback(c.Context(), code, state)
	if err != nil {
		return err
	}

	successURL := strings.TrimSpace(os.Getenv("TIKTOK_OAUTH_SUCCESS_URL"))
	if successURL == "" {
		successURL = "http://localhost:8082/tiktok-setup?connected=1"
	} else if strings.Contains(successURL, "localhost:3000") {
		successURL = strings.Replace(successURL, "localhost:3000", "localhost:8082", 1)
	}
	return c.Redirect(successURL, fiber.StatusFound)
}

// ReceiveWebhook handles incoming TikTok webhook notifications
func (h *TikTokHandler) ReceiveWebhook(c *fiber.Ctx) error {
	eventID := c.Get("X-TikTok-Event-Id")
	timestamp := c.Get("X-TikTok-Timestamp")
	signature := c.Get("X-TikTok-Signature")
	eventType := c.Get("X-TikTok-Event-Type")
	payload := c.Body()

	if err := h.usecase.ReceiveWebhook(c.Context(), eventID, eventType, timestamp, signature, payload); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusOK)
}

// SyncOrders pulls recent orders from TikTok Shop API
func (h *TikTokHandler) SyncOrders(c *fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "30"))
	res, err := h.usecase.SyncOrders(c.Context(), days)
	if err != nil {
		return err
	}
	return response.OK(c, res, "TikTok orders synchronized successfully")
}

// GetSyncRuns returns sync history runs
func (h *TikTokHandler) GetSyncRuns(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	runs, err := h.usecase.GetSyncRuns(c.Context(), limit)
	if err != nil {
		return err
	}
	return response.OK(c, runs)
}

// GetOrders returns recent TikTok orders
func (h *TikTokHandler) GetOrders(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	orders, err := h.usecase.GetOrders(c.Context(), limit)
	if err != nil {
		return err
	}
	return response.OK(c, orders)
}

// GetProducts searches products on TikTok Shop
func (h *TikTokHandler) GetProducts(c *fiber.Ctx) error {
	pageToken := c.Query("page_token", "")
	pageSize, _ := strconv.Atoi(c.Query("page_size", "50"))
	products, err := h.usecase.GetProducts(c.Context(), pageToken, pageSize)
	if err != nil {
		return err
	}
	return response.OK(c, products)
}

// GetStockSyncPreview compares warehouse inventory with TikTok inventory
func (h *TikTokHandler) GetStockSyncPreview(c *fiber.Ctx) error {
	preview, err := h.usecase.GetStockSyncPreview(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, preview)
}

// SaveMapping binds a TikTok Seller SKU to an ERP SKU
func (h *TikTokHandler) SaveMapping(c *fiber.Ctx) error {
	var req struct {
		TikTokSKU    string `json:"tiktok_sku"`
		LocalSKU     string `json:"local_sku"`
		TikTokSKUAlt string `json:"tiktokSku"`
		ERPSKU       string `json:"erpSku"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	tiktokSKU := req.TikTokSKU
	if tiktokSKU == "" && req.TikTokSKUAlt != "" {
		tiktokSKU = req.TikTokSKUAlt
	}

	localSKU := req.LocalSKU
	if localSKU == "" && req.ERPSKU != "" {
		localSKU = req.ERPSKU
	}

	tiktokSKU = strings.TrimSpace(tiktokSKU)
	localSKU = strings.TrimSpace(localSKU)

	if tiktokSKU == "" || localSKU == "" {
		return response.BadRequest(c, "tiktok_sku (tiktokSku) and local_sku (erpSku) are required")
	}

	if err := h.usecase.SaveSKUMapping(c.Context(), usecaseTikTok.MapSKUInput{
		TikTokSKU: tiktokSKU,
		LocalSKU:  localSKU,
	}); err != nil {
		return err
	}

	saved := fiber.Map{
		"tiktokSku": tiktokSKU,
		"erpSku":    localSKU,
		"ratio":     1,
	}

	return response.OK(c, saved, "SKU mapping saved successfully")
}

// ListMappings lists all SKU bindings
func (h *TikTokHandler) ListMappings(c *fiber.Ctx) error {
	mappings, err := h.usecase.ListSKUMappings(c.Context())
	if err != nil {
		return err
	}

	result := make([]fiber.Map, len(mappings))
	for i, m := range mappings {
		result[i] = fiber.Map{
			"id":        m.ID,
			"tiktokSku": m.TikTokSKU,
			"erpSku":    m.LocalSKU,
			"ratio":     1,
			"createdAt": m.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	return response.OK(c, result)
}

// SyncOrder handles manual order ingestion
func (h *TikTokHandler) SyncOrder(c *fiber.Ctx) error {
	var req usecaseTikTok.SyncOrderInput
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.TikTokOrderID == "" {
		return response.BadRequest(c, "tiktok_order_id is required")
	}

	if err := h.usecase.SyncOrder(c.Context(), req); err != nil {
		return err
	}

	return response.OK(c, nil, "TikTok order synced successfully")
}

// GetSyncLogs returns manual sync log history
func (h *TikTokHandler) GetSyncLogs(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	logs, err := h.usecase.GetSyncLogs(c.Context(), limit)
	if err != nil {
		return err
	}
	return response.OK(c, logs)
}
