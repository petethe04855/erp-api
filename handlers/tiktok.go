package handlers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	tiktokAPIBase       = "https://open-api.tiktokglobalshop.com"
	tiktokTokenURL      = "https://auth.tiktok-shops.com/api/v2/token/get"
	tiktokRefreshURL    = "https://auth.tiktok-shops.com/api/v2/token/refresh"
	tiktokRefreshWindow = 24 * time.Hour
)

var errTiktokSKUUnmapped = errors.New("TikTok SKU is not mapped")

type tiktokTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken          string   `json:"access_token"`
		RefreshToken         string   `json:"refresh_token"`
		AccessTokenExpireIn  int64    `json:"access_token_expire_in"`
		RefreshTokenExpireIn int64    `json:"refresh_token_expire_in"`
		ShopCipher           string   `json:"shop_cipher"`
		SellerName           string   `json:"seller_name"`
		SellerBaseRegion     string   `json:"seller_base_region"`
		GrantedScopes        []string `json:"granted_scopes"`
	} `json:"data"`
}

type tiktokOrderSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NextPageToken string `json:"next_page_token"`
		Orders        []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			CreateTime int64  `json:"create_time"`
			Payment    struct {
				TotalAmount string `json:"total_amount"`
			} `json:"payment"`
			LineItems []struct {
				ID          string `json:"id"`
				ProductName string `json:"product_name"`
				SellerSKU   string `json:"seller_sku"`
				Quantity    int    `json:"quantity"`
				SalePrice   string `json:"sale_price"`
			} `json:"line_items"`
		} `json:"orders"`
	} `json:"data"`
}

// SyncTiktokOrders retrieves recent orders from TikTok Shop and upserts the
// order headers and every SKU line into the local reporting database.
func SyncTiktokOrders(c *fiber.Ctx) error {
	appKey, appSecret, err := tiktokCredentials()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	var connection models.TiktokConnection
	if err := database.DB.First(&connection, 1).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "TikTok Shop is not connected"})
	}
	accessToken, err := ensureTiktokAccessToken(&connection, appKey, appSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ensureTiktokShopCipher(&connection, appKey, appSecret); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.Save(&connection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save TikTok shop information"})
	}

	days := c.QueryInt("days", 30)
	if days < 1 || days > 90 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "days must be between 1 and 90"})
	}
	now := time.Now()
	body, _ := json.Marshal(fiber.Map{
		"create_time_ge": now.AddDate(0, 0, -days).Unix(),
		"create_time_le": now.Unix(),
	})
	path := "/order/202309/orders/search"
	pageToken := ""
	orders := make([]models.TiktokOrder, 0)

	for page := 0; page < 20; page++ {
		params := map[string]string{
			"app_key": appKey, "timestamp": fmt.Sprintf("%d", time.Now().Unix()),
			"shop_cipher": connection.ShopCipher, "page_size": "50",
			"sort_field": "create_time", "sort_order": "DESC",
		}
		if pageToken != "" {
			params["page_token"] = pageToken
		}
		params["sign"] = tiktokSignature(path, params, string(body), appSecret)
		request, _ := http.NewRequest(http.MethodPost, tiktokAPIBase+path+"?"+encodeTiktokParams(params), bytes.NewReader(body))
		request.Header.Set("x-tts-access-token", accessToken)
		request.Header.Set("Content-Type", "application/json")
		response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "TikTok Shop Orders API is unavailable"})
		}
		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Could not read TikTok Shop order response"})
		}
		var payload tiktokOrderSearchResponse
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "TikTok Shop returned an invalid order response"})
		}
		if response.StatusCode >= 400 || payload.Code != 0 {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("TikTok Shop error: %s (code %d)", firstNonEmpty(payload.Message, "order sync failed"), payload.Code)})
		}

		for _, source := range payload.Data.Orders {
			items := make([]models.TiktokOrderItem, 0, len(source.LineItems))
			totalQty, itemAmount := 0, 0.0
			for _, line := range source.LineItems {
				qty := line.Quantity
				if qty < 1 {
					qty = 1
				}
				unitPrice, _ := strconv.ParseFloat(line.SalePrice, 64)
				amount := unitPrice * float64(qty)
				totalQty += qty
				itemAmount += amount
				items = append(items, models.TiktokOrderItem{OrderID: source.ID, LineItemID: line.ID, ProductName: line.ProductName, SKU: line.SellerSKU, Qty: qty, UnitPrice: unitPrice, Amount: amount})
			}
			amount, _ := strconv.ParseFloat(source.Payment.TotalAmount, 64)
			if amount == 0 {
				amount = itemAmount
			}
			product, sku := "", ""
			if len(items) > 0 {
				product, sku = items[0].ProductName, items[0].SKU
			}
			if len(items) > 1 {
				product = fmt.Sprintf("%s +%d à¸£à¸²à¸¢à¸à¸²à¸£", product, len(items)-1)
			}
			date := time.Unix(source.CreateTime, 0).In(time.FixedZone("Asia/Bangkok", 7*60*60)).Format("2006-01-02")
			orders = append(orders, models.TiktokOrder{ID: source.ID, Date: date, Product: product, SKU: sku, Qty: totalQty, Amount: amount, Status: source.Status, Items: items})
		}
		pageToken = payload.Data.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		for i := range orders {
			order := &orders[i]
			if err := tx.Omit("Items").Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"date", "product", "sku", "qty", "amount", "status"}),
			}).Create(order).Error; err != nil {
				return err
			}
			if err := tx.Where("order_id = ?", order.ID).Delete(&models.TiktokOrderItem{}).Error; err != nil {
				return err
			}
			if len(order.Items) > 0 {
				if err := tx.Create(&order.Items).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	deducted := 0
	deductionErrors := make([]string, 0)
	deductionWarnings := make([]string, 0)
	for i := range orders {
		order := &orders[i]
		if !tiktokOrderNeedsStockDeduction(order.Status) || order.StockDeducted {
			continue
		}
		deduction, err := deductTiktokOrderStock(order.ID)
		if err != nil {
			deductionErrors = append(deductionErrors, fmt.Sprintf("%s: %s", order.ID, err.Error()))
			continue
		}
		if deduction.DidDeduct {
			deducted++
		}
		for _, warning := range deduction.Warnings {
			deductionWarnings = append(deductionWarnings, fmt.Sprintf("%s: %s", order.ID, warning))
		}
	}
	result := fiber.Map{"synced": len(orders), "days": days, "stockDeducted": deducted}
	if len(deductionErrors) > 0 {
		result["stockDeductionErrors"] = deductionErrors
	}
	if len(deductionWarnings) > 0 {
		result["stockDeductionWarnings"] = deductionWarnings
	}
	return c.JSON(result)
}

func tiktokCredentials() (string, string, error) {
	key, secret := strings.TrimSpace(os.Getenv("TIKTOK_APP_KEY")), strings.TrimSpace(os.Getenv("TIKTOK_APP_SECRET"))
	if key == "" || secret == "" {
		return "", "", errors.New("TikTok is not configured: TIKTOK_APP_KEY and TIKTOK_APP_SECRET are required")
	}
	return key, secret, nil
}

// StartTiktokConnect returns an authorization URL. The caller redirects the
// browser to it; credentials and state never leave the server as application data.
func StartTiktokConnect(c *fiber.Ctx) error {
	_, _, err := tiktokCredentials()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	serviceID := strings.TrimSpace(os.Getenv("TIKTOK_SERVICE_ID"))
	if serviceID == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "TikTok authorization is not configured: TIKTOK_SERVICE_ID is required"})
	}
	state, err := newTiktokState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{"error": "Unable to start TikTok authorization"},
		)
	}

	now := time.Now()
	database.DB.Where("expires_at < ?", now).Delete(&models.TiktokOAuthState{})

	oauthState := models.TiktokOAuthState{
		StateHash: hashTiktokState(state),
		ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := database.DB.Create(&oauthState).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{"error": "Unable to save TikTok authorization state"},
		)
	}

	authorizeURL := os.Getenv("TIKTOK_AUTHORIZE_URL")
	if authorizeURL == "" {
		authorizeURL = "https://services.tiktokshop.com/open/authorize"
	}
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "TIKTOK_AUTHORIZE_URL is invalid"})
	}
	q := u.Query()
	q.Set("service_id", serviceID)
	q.Set("state", state)

	u.RawQuery = q.Encode()
	return c.JSON(fiber.Map{"authorizationUrl": u.String()})
}

// TiktokCallback validates the OAuth state, exchanges the one-time code and
// stores the encrypted connection. Configure this exact callback URL in Partner Center.
func TiktokCallback(c *fiber.Ctx) error {
	code, state := c.Query("code"), c.Query("state")

	if code == "" || state == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "TikTok authorization code and state are required"},
		)
	}

	appKey, appSecret, err := tiktokCredentials()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(
			fiber.Map{"error": err.Error()},
		)
	}

	now := time.Now()
	result := database.DB.Model(&models.TiktokOAuthState{}).
		Where(
			"state_hash = ? AND expires_at > ? AND used_at IS NULL",
			hashTiktokState(state), now,
		).
		Update("used_at", now)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{"error": "Unable to validate TikTok authorization state"},
		)
	}
	if result.RowsAffected != 1 {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "Invalid or expired TikTok authorization response"},
		)
	}

	connection, err := exchangeTiktokCode(appKey, appSecret, code)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ensureTiktokShopCipher(&connection, appKey, appSecret); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.Where("id = ?", 1).Assign(connection).FirstOrCreate(&connection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save TikTok connection"})
	}

	if successURL := strings.TrimSpace(os.Getenv("TIKTOK_OAUTH_SUCCESS_URL")); successURL != "" {
		return c.Redirect(successURL, fiber.StatusFound)
	}
	return c.JSON(fiber.Map{"connected": true, "message": "à¹€à¸Šà¸·à¹ˆà¸­à¸¡à¸•à¹ˆà¸­ TikTok Shop à¸ªà¸³à¹€à¸£à¹‡à¸ˆ", "shopCipher": connection.ShopCipher, "sellerName": connection.SellerName})
}

func GetTiktokConnection(c *fiber.Ctx) error {
	var connection models.TiktokConnection
	err := database.DB.First(&connection, 1).Error
	if err != nil {
		return c.JSON(fiber.Map{"connected": false})
	}
	return c.JSON(fiber.Map{
		"connected":             true,
		"needsReauthorization":  !connection.RefreshTokenExpiresAt.After(time.Now()),
		"shopCipher":            connection.ShopCipher,
		"sellerName":            connection.SellerName,
		"sellerBaseRegion":      connection.SellerBaseRegion,
		"grantedScopes":         connection.GrantedScopes,
		"accessTokenExpiresAt":  connection.AccessTokenExpiresAt,
		"refreshTokenExpiresAt": connection.RefreshTokenExpiresAt,
	})
}

func GetTiktokProducts(c *fiber.Ctx) error {
	appKey, appSecret, err := tiktokCredentials()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	var connection models.TiktokConnection
	if err := database.DB.First(&connection, 1).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "TikTok Shop is not connected"})
	}
	accessToken, err := ensureTiktokAccessToken(&connection, appKey, appSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ensureTiktokShopCipher(&connection, appKey, appSecret); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.Save(&connection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save TikTok shop information"})
	}

	pageSize := c.QueryInt("page_size", 20)
	if pageSize < 1 || pageSize > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "page_size must be between 1 and 100"})
	}
	body, _ := json.Marshal(fiber.Map{"status": firstNonEmpty(c.Query("status"), "ACTIVATE")})
	path := "/product/202309/products/search"
	params := map[string]string{"app_key": appKey, "timestamp": fmt.Sprintf("%d", time.Now().Unix()), "shop_cipher": connection.ShopCipher, "page_size": fmt.Sprint(pageSize)}
	params["sign"] = tiktokSignature(path, params, string(body), appSecret)
	req, _ := http.NewRequest(http.MethodPost, tiktokAPIBase+path+"?"+encodeTiktokParams(params), bytes.NewReader(body))
	req.Header.Set("x-tts-access-token", accessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "TikTok Shop API is unavailable"})
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Could not read TikTok Shop response"})
	}
	var upstream struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	decodeErr := json.Unmarshal(responseBody, &upstream)
	if response.StatusCode >= 400 {
		if decodeErr == nil && (upstream.Code != 0 || upstream.Message != "") {
			message := firstNonEmpty(upstream.Message, "unknown error")
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("TikTok Shop error: %s (code %d)", message, upstream.Code)})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("TikTok Shop rejected the product request (HTTP %d)", response.StatusCode)})
	}
	if decodeErr != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "TikTok Shop returned an invalid product response"})
	}
	if upstream.Code != 0 {
		message := firstNonEmpty(upstream.Message, "unknown error")
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("TikTok Shop error: %s (code %d)", message, upstream.Code)})
	}
	if c.Query("debug") == "true" {
		return c.JSON(fiber.Map{
			"response": json.RawMessage(responseBody),
			"outgoingRequest": fiber.Map{
				"method": http.MethodPost,
				"path":   path,
				"query": fiber.Map{
					"app_key":     redactTiktokValue(appKey),
					"timestamp":   params["timestamp"],
					"shop_cipher": redactTiktokValue(connection.ShopCipher),
					"page_size":   params["page_size"],
					"sign":        "[redacted]",
				},
				"headers": fiber.Map{
					"x-tts-access-token": "[redacted]",
					"content-type":       "application/json",
				},
				"body": json.RawMessage(body),
			},
		})
	}
	return c.Type(fiber.MIMEApplicationJSON).Send(responseBody)
}

func redactTiktokValue(value string) string {
	if len(value) <= 4 {
		return "[redacted]"
	}
	return value[:2] + "â€¦" + value[len(value)-2:]
}

func exchangeTiktokCode(appKey, appSecret, code string) (models.TiktokConnection, error) {
	u, _ := url.Parse(tiktokTokenURL)
	q := u.Query()
	q.Set("app_key", appKey)
	q.Set("app_secret", appSecret)
	q.Set("auth_code", code)
	q.Set("grant_type", "authorized_code")
	u.RawQuery = q.Encode()
	response, err := (&http.Client{Timeout: 20 * time.Second}).Get(u.String())
	if err != nil {
		return models.TiktokConnection{}, errors.New("TikTok token exchange is unavailable")
	}
	defer response.Body.Close()
	var result tiktokTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return models.TiktokConnection{}, errors.New("TikTok returned an invalid token response")
	}
	if response.StatusCode >= 400 || result.Code != 0 || result.Data.AccessToken == "" {
		return models.TiktokConnection{}, fmt.Errorf("TikTok token exchange failed: %s", firstNonEmpty(result.Message, "unknown error"))
	}
	accessToken, err := encryptTiktokToken(result.Data.AccessToken)
	if err != nil {
		return models.TiktokConnection{}, err
	}
	refreshToken, err := encryptTiktokToken(result.Data.RefreshToken)
	if err != nil {
		return models.TiktokConnection{}, err
	}
	return models.TiktokConnection{ID: 1, AccessToken: accessToken, RefreshToken: refreshToken, AccessTokenExpiresAt: time.Unix(result.Data.AccessTokenExpireIn, 0), RefreshTokenExpiresAt: time.Unix(result.Data.RefreshTokenExpireIn, 0), ShopCipher: result.Data.ShopCipher, SellerName: result.Data.SellerName, SellerBaseRegion: result.Data.SellerBaseRegion, GrantedScopes: strings.Join(result.Data.GrantedScopes, ",")}, nil
}

// ensureTiktokShopCipher resolves the target shop after OAuth. The token
// response does not always include shop_cipher, but product APIs require it.
func ensureTiktokShopCipher(connection *models.TiktokConnection, appKey, appSecret string) error {
	if connection.ShopCipher != "" {
		return nil
	}
	accessToken, err := decryptTiktokToken(connection.AccessToken)
	if err != nil {
		return errors.New("Could not read TikTok credentials")
	}
	path := "/authorization/202309/shops"
	params := map[string]string{
		"app_key":   appKey,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}
	params["sign"] = tiktokSignature(path, params, "", appSecret)
	values := encodeTiktokParams(params)
	requestURL := fmt.Sprintf("%s%s?%s&access_token=%s", tiktokAPIBase, path, values, url.QueryEscape(accessToken))
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return errors.New("Could not create TikTok shop request")
	}
	req.Header.Set("x-tts-access-token", accessToken)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return errors.New("TikTok Shop API is unavailable")
	}
	defer response.Body.Close()

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Shops []struct {
				Cipher string `json:"cipher"`
				Name   string `json:"name"`
			} `json:"shops"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return errors.New("TikTok returned an invalid authorized shops response")
	}
	if response.StatusCode >= 400 || payload.Code != 0 {
		return fmt.Errorf("TikTok Shop error: %s (code %d)", firstNonEmpty(payload.Message, "could not retrieve authorized shops"), payload.Code)
	}
	if len(payload.Data.Shops) == 0 || payload.Data.Shops[0].Cipher == "" {
		return errors.New("No authorized TikTok Shop was found")
	}
	connection.ShopCipher = payload.Data.Shops[0].Cipher
	if connection.SellerName == "" {
		connection.SellerName = payload.Data.Shops[0].Name
	}
	return nil
}

// ensureTiktokAccessToken refreshes server-side before the access token expires.
// The user only needs to authorize again after the refresh token expires or is revoked.
func ensureTiktokAccessToken(connection *models.TiktokConnection, appKey, appSecret string) (string, error) {
	if time.Until(connection.AccessTokenExpiresAt) > tiktokRefreshWindow {
		return decryptTiktokToken(connection.AccessToken)
	}
	if !connection.RefreshTokenExpiresAt.After(time.Now()) {
		return "", errors.New("TikTok authorization has expired; reconnect the shop")
	}

	refreshToken, err := decryptTiktokToken(connection.RefreshToken)
	if err != nil {
		return "", errors.New("Could not read TikTok refresh credentials")
	}
	result, err := refreshTiktokToken(appKey, appSecret, refreshToken)
	if err != nil {
		return "", err
	}
	accessToken, err := encryptTiktokToken(result.Data.AccessToken)
	if err != nil {
		return "", err
	}
	connection.AccessToken = accessToken
	connection.AccessTokenExpiresAt = time.Unix(result.Data.AccessTokenExpireIn, 0)
	if result.Data.RefreshToken != "" {
		encryptedRefreshToken, err := encryptTiktokToken(result.Data.RefreshToken)
		if err != nil {
			return "", err
		}
		connection.RefreshToken = encryptedRefreshToken
	}
	if result.Data.RefreshTokenExpireIn > 0 {
		connection.RefreshTokenExpiresAt = time.Unix(result.Data.RefreshTokenExpireIn, 0)
	}
	if err := database.DB.Save(connection).Error; err != nil {
		return "", errors.New("Could not save refreshed TikTok credentials")
	}
	return result.Data.AccessToken, nil
}

func refreshTiktokToken(appKey, appSecret, refreshToken string) (tiktokTokenResponse, error) {
	u, _ := url.Parse(tiktokRefreshURL)
	q := u.Query()
	q.Set("app_key", appKey)
	q.Set("app_secret", appSecret)
	q.Set("refresh_token", refreshToken)
	q.Set("grant_type", "refresh_token")
	u.RawQuery = q.Encode()

	response, err := (&http.Client{Timeout: 20 * time.Second}).Get(u.String())
	if err != nil {
		return tiktokTokenResponse{}, errors.New("TikTok token refresh is unavailable")
	}
	defer response.Body.Close()
	var result tiktokTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return tiktokTokenResponse{}, errors.New("TikTok returned an invalid refresh response")
	}
	if response.StatusCode >= 400 || result.Code != 0 || result.Data.AccessToken == "" || result.Data.AccessTokenExpireIn == 0 {
		return tiktokTokenResponse{}, errors.New("TikTok authorization has expired; reconnect the shop")
	}
	return result, nil
}

func tiktokSignature(path string, params map[string]string, body, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "sign" && key != "access_token" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	value := secret + path
	for _, key := range keys {
		value += key + params[key]
	}
	value += body + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func encodeTiktokParams(params map[string]string) string {
	values := make(url.Values, len(params))
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}
func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func newTiktokState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashTiktokState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

func tiktokCipher() (cipher.AEAD, error) {
	encoded := os.Getenv("TIKTOK_TOKEN_ENCRYPTION_KEY")
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("TIKTOK_TOKEN_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func encryptTiktokToken(value string) (string, error) {
	aead, err := tiktokCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(nonce, aead.Seal(nil, nonce, []byte(value), nil)...)), nil
}
func decryptTiktokToken(value string) (string, error) {
	aead, err := tiktokCipher()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) < aead.NonceSize() {
		return "", errors.New("invalid encrypted TikTok token")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	return string(plain), err
}

func normalizeTiktokStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func tiktokOrderNeedsStockDeduction(status string) bool {
	// Stock is consumed when TikTok has confirmed the order has left the seller's
	// fulfillment flow. Keep this centralized so status variants are handled consistently.
	switch normalizeTiktokStatus(status) {
	case "SHIPPED", "IN_TRANSIT", "DELIVERED", "COMPLETED":
		return true
	default:
		return false
	}
}

func resolveTiktokERPSKU(tx *gorm.DB, tiktokSKU string) (string, error) {
	sku := strings.ToUpper(strings.TrimSpace(tiktokSKU))
	if sku == "" {
		return "", fmt.Errorf("%w: Seller SKU is empty", errTiktokSKUUnmapped)
	}
	var mapping models.TiktokSKUMapping
	if err := tx.Where("tiktok_sku = ?", sku).First(&mapping).Error; err == nil {
		return strings.ToUpper(strings.TrimSpace(mapping.ERPSKU)), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	// Backward-compatible fallback: existing installations where TikTok Seller SKU
	// already equals the ERP SKU continue to work without a mapping row.
	var product models.Product
	if err := tx.First(&product, "sku = ?", sku).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("%w: TikTok SKU %s has no ERP product or mapping", errTiktokSKUUnmapped, sku)
		}
		return "", err
	}
	return product.SKU, nil
}

type tiktokStockDeductionResult struct {
	DidDeduct bool
	Warnings  []string
}

// deductTiktokOrderStock consumes ERP stock when a TikTok order is delivered.
// Bundle SKUs are expanded into their component SKUs; normal SKUs are consumed directly.
// The whole order is processed in one transaction and StockDeducted makes the operation idempotent.
func deductTiktokOrderStock(orderID string) (tiktokStockDeductionResult, error) {
	result := tiktokStockDeductionResult{Warnings: make([]string, 0)}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order models.TiktokOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
			return fmt.Errorf("TikTok order not found")
		}
		if order.StockDeducted || !tiktokOrderNeedsStockDeduction(order.Status) {
			return nil
		}
		if len(order.Items) == 0 {
			return fmt.Errorf("order has no items")
		}

		requirements := make(map[string]int)
		for _, item := range order.Items {
			if item.Qty <= 0 {
				return fmt.Errorf("invalid quantity for SKU %s", item.SKU)
			}
			erpSKU, err := resolveTiktokERPSKU(tx, item.SKU)
			if err != nil {
				if shouldSkipUnmappedTiktokItem(item, err) {
					result.Warnings = append(result.Warnings, fmt.Sprintf("ข้ามรายการมูลค่า 0 ที่ไม่ผูก Stock: %s (%s)", firstNonEmpty(item.ProductName, "ไม่ระบุชื่อ"), firstNonEmpty(strings.TrimSpace(item.SKU), "ไม่มี Seller SKU")))
					continue
				}
				return err
			}
			if err := expandTiktokSKU(tx, erpSKU, item.Qty, requirements, map[string]bool{}); err != nil {
				return err
			}
		}
		if len(requirements) == 0 {
			return fmt.Errorf("order has no stock-tracked items")
		}
		for sku, qty := range requirements {
			if err := deductTiktokComponentFEFO(tx, sku, qty, order.ID); err != nil {
				return err
			}
		}
		if err := tx.Model(&order).Update("stock_deducted", true).Error; err != nil {
			return err
		}
		result.DidDeduct = true
		return nil
	})
	if err != nil {
		result.DidDeduct = false
		result.Warnings = nil
	}
	return result, err
}

func expandTiktokSKU(tx *gorm.DB, sku string, qty int, requirements map[string]int, visiting map[string]bool) error {
	var product models.Product
	if err := tx.First(&product, "sku = ?", sku).Error; err != nil {
		return fmt.Errorf("ERP SKU %s not found", sku)
	}
	if !product.IsBundle && product.Type != "Bundle" {
		requirements[sku] += qty
		return nil
	}
	if visiting[sku] {
		return fmt.Errorf("circular bundle detected at %s", sku)
	}
	visiting[sku] = true
	defer delete(visiting, sku)
	var components []models.BundleComponent
	if err := tx.Where("bundle_sku = ?", sku).Find(&components).Error; err != nil {
		return err
	}
	if len(components) == 0 {
		return fmt.Errorf("bundle %s has no components", sku)
	}
	for _, component := range components {
		componentQty := int(math.Ceil(component.Qty * float64(qty)))
		componentProductSKU := ""
		if component.ComponentProductID != 0 {
			var componentProduct models.Product
			if err := tx.First(&componentProduct, component.ComponentProductID).Error; err != nil {
				return fmt.Errorf("bundle %s component product %d could not be resolved: %w", sku, component.ComponentProductID, err)
			}
			componentProductSKU = componentProduct.SKU
		}
		componentSKU, err := resolveTiktokBundleComponentSKU(sku, component, componentProductSKU)
		if err != nil {
			return err
		}
		if componentQty <= 0 {
			return fmt.Errorf("bundle %s has invalid component quantity for %s", sku, componentSKU)
		}
		if err := expandTiktokSKU(tx, componentSKU, componentQty, requirements, visiting); err != nil {
			return fmt.Errorf("bundle %s component %s: %w", sku, componentSKU, err)
		}
	}
	return nil
}

func shouldSkipUnmappedTiktokItem(item models.TiktokOrderItem, err error) bool {
	return item.Amount == 0 && errors.Is(err, errTiktokSKUUnmapped)
}

func resolveTiktokBundleComponentSKU(bundleSKU string, component models.BundleComponent, productSKU string) (string, error) {
	storedSKU := strings.ToUpper(strings.TrimSpace(component.ComponentSku))
	resolvedSKU := strings.ToUpper(strings.TrimSpace(productSKU))
	if component.ComponentProductID != 0 {
		if resolvedSKU == "" {
			return "", fmt.Errorf("bundle %s component product %d has no valid ERP SKU", bundleSKU, component.ComponentProductID)
		}
		if storedSKU != "" && storedSKU != resolvedSKU {
			return "", fmt.Errorf("bundle %s component data mismatch: product %d is SKU %s but component stores SKU %s", bundleSKU, component.ComponentProductID, resolvedSKU, storedSKU)
		}
		return resolvedSKU, nil
	}
	if storedSKU == "" {
		return "", fmt.Errorf("bundle %s has component with no valid ERP SKU", bundleSKU)
	}
	return storedSKU, nil
}

func deductTiktokComponentFEFO(tx *gorm.DB, sku string, qty int, orderID string) error {
	var product models.Product
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, "sku = ?", sku).Error; err != nil {
		return fmt.Errorf("ERP SKU %s not found", sku)
	}
	if product.IsBundle || product.Type == "Bundle" {
		return fmt.Errorf("bundle %s cannot hold physical stock", sku)
	}
	if product.Stock-product.ReservedQty < qty {
		return fmt.Errorf("stock not sufficient for %s: need %d, available %d", sku, qty, product.Stock-product.ReservedQty)
	}
	if err := ensureLotBalance(tx, product); err != nil {
		return err
	}
	var lots []models.StockLot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sku = ? AND remaining_qty > 0", sku).
		Order("CASE WHEN expiry_date IS NULL OR expiry_date = '' THEN 1 ELSE 0 END, expiry_date ASC, received_date ASC, id ASC").Find(&lots).Error; err != nil {
		return err
	}
	remaining := qty
	for i := range lots {
		if remaining == 0 {
			break
		}
		deduct := lots[i].RemainingQty
		if deduct > remaining {
			deduct = remaining
		}
		lots[i].RemainingQty -= deduct
		remaining -= deduct
		if err := tx.Save(&lots[i]).Error; err != nil {
			return err
		}
		movement := models.StockMovement{
			Code: fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), sku), ProductID: product.ID, SKU: sku,
			Type: "OUT", Qty: deduct, RefDoc: orderID, RefDocType: "tiktok_orders", RefDocID: nil,
			Date: time.Now().Format("2006-01-02"), Note: fmt.Sprintf("TikTok delivered FEFO lot %s", lots[i].Lot), ChangedBy: "TikTok Sync",
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
	}
	if remaining > 0 {
		return fmt.Errorf("lot stock not sufficient for %s: missing %d", sku, remaining)
	}
	product.Stock -= qty
	if err := tx.Save(&product).Error; err != nil {
		return err
	}
	return nil
}
