package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"chawy-erp-api/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewConnection(cfg *config.Config) (*gorm.DB, error) {
	var dsn string
	if cfg.DatabaseURL != "" {
		dsn = cfg.DatabaseURL
		log.Printf("[INFO] Connecting to database using DATABASE_URL (target DB: %s)", cfg.DBName)
	} else {
		hasPass := "no"
		if cfg.DBPassword != "" {
			hasPass = "yes"
		}
		log.Printf("[INFO] Connecting to database: host=%s port=%s user=%s dbname=%s (password configured: %s)",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBName, hasPass)

		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Bangkok",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
		)
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		// If database does not exist (SQLSTATE 3D000), attempt auto-creation
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "3D000") {
			log.Printf("[INFO] Database '%s' does not exist. Creating database now...", cfg.DBName)
			if createErr := createDatabase(cfg); createErr == nil {
				db, err = gorm.Open(postgres.Open(dsn), gormConfig)
			} else {
				log.Printf("[ERROR] Auto-creating database failed: %v", createErr)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Printf("[INFO] Successfully connected to PostgreSQL database '%s'", cfg.DBName)
	return db, nil
}

func createDatabase(cfg *config.Config) error {
	defaultDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s TimeZone=Asia/Bangkok",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBSSLMode,
	)
	if cfg.DatabaseURL != "" {
		if u, err := url.Parse(cfg.DatabaseURL); err == nil && u.Scheme != "" {
			u.Path = "/postgres"
			defaultDSN = u.String()
		}
	}

	tempDB, err := gorm.Open(postgres.Open(defaultDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	sqlDB, err := tempDB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	targetDB := cfg.DBName
	if targetDB == "" {
		targetDB = "chawy_erp_v2"
	}

	if err := tempDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, targetDB)).Error; err != nil {
		return err
	}
	log.Printf("[INFO] Database '%s' created successfully", targetDB)
	return nil
}

type txKey struct{}

// WithTxContext injects a transaction DB instance into context
func WithTxContext(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// GetDBFromContext retrieves the transaction DB if present, otherwise returns fallback
func GetDBFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}

// TxManager abstracts transaction coordination so usecases stay decoupled from GORM.
type TxManager interface {
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

type gormTxManager struct{ db *gorm.DB }

// NewTxManager returns a TxManager backed by the given GORM handle.
func NewTxManager(db *gorm.DB) TxManager { return &gormTxManager{db: db} }

func (m *gormTxManager) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(WithTxContext(ctx, tx))
	})
}
