package tiktok

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"
)

const (
	APIBaseURL       = "https://open-api.tiktokglobalshop.com"
	TokenURL         = "https://auth.tiktok-shops.com/api/v2/token/get"
	RefreshURL       = "https://auth.tiktok-shops.com/api/v2/token/refresh"
	MaxRetries       = 3
	TokenExpireGrace = 24 * time.Hour
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type TokenData struct {
	AccessToken          string   `json:"access_token"`
	RefreshToken         string   `json:"refresh_token"`
	AccessTokenExpireIn  int64    `json:"access_token_expire_in"`
	RefreshTokenExpireIn int64    `json:"refresh_token_expire_in"`
	ShopCipher           string   `json:"shop_cipher"`
	SellerName           string   `json:"seller_name"`
	SellerBaseRegion     string   `json:"seller_base_region"`
	GrantedScopes        []string `json:"granted_scopes"`
}

type TokenResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    TokenData `json:"data"`
}

type ShopItem struct {
	Cipher string `json:"cipher"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type AuthorizedShopsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Shops []ShopItem `json:"shops"`
	} `json:"data"`
}

type OrderLineItem struct {
	ID          string `json:"id"`
	ProductName string `json:"product_name"`
	SellerSKU   string `json:"seller_sku"`
	Quantity    int    `json:"quantity"`
	SalePrice   string `json:"sale_price"`
}

type OrderItem struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	CreateTime int64  `json:"create_time"`
	Payment    struct {
		TotalAmount string `json:"total_amount"`
	} `json:"payment"`
	LineItems []OrderLineItem `json:"line_items"`
}

type OrderSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NextPageToken string      `json:"next_page_token"`
		Orders        []OrderItem `json:"orders"`
		TotalCount    int         `json:"total_count"`
	} `json:"data"`
}

type ProductSKU struct {
	ID         string `json:"id"`
	SellerSKU  string `json:"seller_sku"`
	Price      string `json:"price"`
	StockInfos []struct {
		AvailableStock int `json:"available_stock"`
	} `json:"stock_infos"`
}

type ProductItem struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Status      string       `json:"status"`
	SKUs        []ProductSKU `json:"skus"`
	CreateTime  int64        `json:"create_time"`
	UpdateTime  int64        `json:"update_time"`
}

type ProductSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NextPageToken string        `json:"next_page_token"`
		Products      []ProductItem `json:"products"`
		TotalCount    int           `json:"total_count"`
	} `json:"data"`
}

// GenerateSignature calculates the HMAC-SHA256 signature for TikTok Open API requests
func GenerateSignature(path string, params map[string]string, body string, secret string) string {
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

func encodeParams(params map[string]string) string {
	values := make(url.Values, len(params))
	for k, v := range params {
		values.Set(k, v)
	}
	return values.Encode()
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		current := req
		if attempt > 0 {
			current = req.Clone(req.Context())
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				current.Body = body
			}
		}

		resp, err := c.httpClient.Do(current)
		if err == nil && (resp.StatusCode < 429 || attempt == MaxRetries-1) {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
		if attempt < MaxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * 300 * time.Millisecond)
		}
	}
	return nil, lastErr
}

// ExchangeCode exchanges the OAuth authorization code for access and refresh tokens
func (c *Client) ExchangeCode(appKey, appSecret, code string) (*TokenResponse, error) {
	u, _ := url.Parse(TokenURL)
	q := u.Query()
	q.Set("app_key", appKey)
	q.Set("app_secret", appSecret)
	q.Set("auth_code", code)
	q.Set("grant_type", "authorized_code")
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("tiktok token request failed: %w", err)
	}
	defer resp.Body.Close()

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	if resp.StatusCode >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("tiktok auth error: %s (code %d)", result.Message, result.Code)
	}
	return &result, nil
}

// RefreshToken refreshes an expired or expiring access token
func (c *Client) RefreshToken(appKey, appSecret, refreshToken string) (*TokenResponse, error) {
	u, _ := url.Parse(RefreshURL)
	q := u.Query()
	q.Set("app_key", appKey)
	q.Set("app_secret", appSecret)
	q.Set("refresh_token", refreshToken)
	q.Set("grant_type", "refresh_token")
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("tiktok token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}
	if resp.StatusCode >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("tiktok refresh error: %s (code %d)", result.Message, result.Code)
	}
	return &result, nil
}

// GetAuthorizedShops retrieves the authorized shop cipher for a given token
func (c *Client) GetAuthorizedShops(appKey, appSecret, accessToken string) (*AuthorizedShopsResponse, error) {
	path := "/authorization/202309/shops"
	params := map[string]string{
		"app_key":   appKey,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}
	params["sign"] = GenerateSignature(path, params, "", appSecret)
	query := encodeParams(params)
	requestURL := fmt.Sprintf("%s%s?%s&access_token=%s", APIBaseURL, path, query, url.QueryEscape(accessToken))

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-tts-access-token", accessToken)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch authorized shops: %w", err)
	}
	defer resp.Body.Close()

	var result AuthorizedShopsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode shops response: %w", err)
	}
	if resp.StatusCode >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("tiktok shop list error: %s (code %d)", result.Message, result.Code)
	}
	return &result, nil
}

// SearchOrders queries TikTok Shop for orders within a timeframe
func (c *Client) SearchOrders(accessToken, shopCipher, appKey, appSecret string, startTime, endTime int64, pageToken string, pageSize int) (*OrderSearchResponse, error) {
	path := "/order/202309/orders/search"
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}

	bodyMap := map[string]interface{}{
		"create_time_ge": startTime,
		"create_time_le": endTime,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	params := map[string]string{
		"app_key":     appKey,
		"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		"shop_cipher": shopCipher,
		"page_size":   fmt.Sprintf("%d", pageSize),
		"sort_field":  "create_time",
		"sort_order":  "DESC",
	}
	if pageToken != "" {
		params["page_token"] = pageToken
	}
	params["sign"] = GenerateSignature(path, params, string(bodyBytes), appSecret)

	reqURL := fmt.Sprintf("%s%s?%s", APIBaseURL, path, encodeParams(params))
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-tts-access-token", accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok orders search request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyResp, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read orders response: %w", readErr)
	}

	var result OrderSearchResponse
	if err := json.Unmarshal(bodyResp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal orders response: %w", err)
	}
	if resp.StatusCode >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("tiktok orders API error: %s (code %d)", result.Message, result.Code)
	}
	return &result, nil
}

// SearchProducts queries products from TikTok Shop
func (c *Client) SearchProducts(accessToken, shopCipher, appKey, appSecret string, pageToken string, pageSize int) (*ProductSearchResponse, error) {
	path := "/product/202309/products/search"
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}

	bodyBytes := []byte("{}")
	params := map[string]string{
		"app_key":     appKey,
		"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		"shop_cipher": shopCipher,
		"page_size":   fmt.Sprintf("%d", pageSize),
	}
	if pageToken != "" {
		params["page_token"] = pageToken
	}
	params["sign"] = GenerateSignature(path, params, string(bodyBytes), appSecret)

	reqURL := fmt.Sprintf("%s%s?%s", APIBaseURL, path, encodeParams(params))
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-tts-access-token", accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok products search request failed: %w", err)
	}
	defer resp.Body.Close()

	var result ProductSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode products response: %w", err)
	}
	if resp.StatusCode >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("tiktok products API error: %s (code %d)", result.Message, result.Code)
	}
	return &result, nil
}

// VerifyWebhookSignature checks the HMAC-SHA256 signature from TikTok Webhook header
func VerifyWebhookSignature(eventID, timestamp, signature, body, secret string) error {
	if secret == "" {
		return errors.New("webhook secret is not configured")
	}
	if eventID == "" || timestamp == "" || signature == "" {
		return errors.New("missing webhook authentication headers")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write([]byte(body))
	expected := hex.EncodeToString(mac.Sum(nil))

	cleanSignature := signature
	if len(cleanSignature) > 7 && cleanSignature[:7] == "sha256=" {
		cleanSignature = cleanSignature[7:]
	}

	if !hmac.Equal([]byte(cleanSignature), []byte(expected)) {
		return errors.New("invalid webhook signature")
	}
	return nil
}
