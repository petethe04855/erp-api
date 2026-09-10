package config

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	DatabaseURL string
	JWTSecret   string
	JWTExpHours string

	// TikTok Shop Integration
	TikTokAppKey              string
	TikTokAppSecret           string
	TikTokServiceID           string
	TikTokAuthorizeURL        string
	TikTokOAuthSuccessURL     string
	TikTokWebhookSecret       string
	TikTokTokenEncryptionKey  string
	TikTokSyncIntervalMinutes string
	TikTokSyncToken           string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("[INFO] No .env file found in current or parent directory, using system environment variables")
		} else {
			log.Println("[INFO] Successfully loaded .env from parent directory")
		}
	} else {
		log.Println("[INFO] Successfully loaded .env from current directory")
	}

	dbPassword := getEnv("DB_PASSWORD", "")
	if dbPassword == "" {
		dbPassword = getEnv("POSTGRES_PASSWORD", "")
	}
	if dbPassword == "" {
		dbPassword = getEnv("DB_PASS", "")
	}

	dbUser := getEnv("DB_USER", "")
	if dbUser == "" {
		dbUser = getEnv("POSTGRES_USER", "postgres")
	}

	// FULL-05: check DB_NAME first (empty means unset), then POSTGRES_DB, then default
	dbName := getEnv("DB_NAME", "")
	if dbName == "" {
		dbName = getEnv("POSTGRES_DB", "chawy_erp_v2")
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL != "" && dbName != "" {
		// If DATABASE_URL is provided, make sure its database matches DB_NAME
		if u, err := url.Parse(databaseURL); err == nil && u.Scheme != "" {
			if u.Path == "" || u.Path == "/" || u.Path == "/chawy_erp" || u.Path == "/postgres" {
				u.Path = "/" + dbName
				databaseURL = u.String()
			}
		}
	}

	return &Config{
		Port:        getEnv("PORT", "8084"),
		Environment: getEnv("ENV", "development"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBName:      dbName,
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		DatabaseURL: databaseURL,
		// FULL-02: no hardcoded fallback secret; startup validation lives in main
		JWTSecret:   getEnv("JWT_SECRET", ""),
		JWTExpHours: getEnv("JWT_EXPIRATION_HOURS", "24"),

		TikTokAppKey:              getEnv("TIKTOK_APP_KEY", ""),
		TikTokAppSecret:           getEnv("TIKTOK_APP_SECRET", ""),
		TikTokServiceID:           getEnv("TIKTOK_SERVICE_ID", ""),
		TikTokAuthorizeURL:        getEnv("TIKTOK_AUTHORIZE_URL", "https://services.tiktokshop.com/open/authorize"),
		TikTokOAuthSuccessURL:     getEnv("TIKTOK_OAUTH_SUCCESS_URL", ""),
		TikTokWebhookSecret:       getEnv("TIKTOK_WEBHOOK_SECRET", ""),
		TikTokTokenEncryptionKey:  getEnv("TIKTOK_TOKEN_ENCRYPTION_KEY", ""),
		TikTokSyncIntervalMinutes: getEnv("TIKTOK_SYNC_INTERVAL_MINUTES", "30"),
		TikTokSyncToken:           getEnv("TIKTOK_SYNC_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

// Validate rejects configurations that are unsafe to serve traffic with.
// FULL-02: production must supply a strong JWT secret; the old default made
// tokens forgeable by anyone who read the source.
func (c *Config) Validate() error {
	if c.Environment == "production" {
		if len(c.JWTSecret) < 32 {
			return fmt.Errorf("JWT_SECRET must be set to at least 32 characters in production")
		}
	}
	return nil
}
