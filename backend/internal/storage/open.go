package storage

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"deezmails/internal/config"
)

func Open(ctx context.Context, appConfig config.Config) (*gorm.DB, error) {
	if appConfig.DatabaseURL != "" {
		db, err := gorm.Open(postgres.Open(appConfig.DatabaseURL), databaseGormConfig())
		if err != nil {
			return nil, fmt.Errorf("open PostgreSQL database: %w", err)
		}
		if err := configureDBPool(db, 20, 10); err != nil {
			return nil, err
		}

		return db, nil
	}

	path, err := filepath.Abs(appConfig.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite path: %w", err)
	}
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return nil, fmt.Errorf("create SQLite directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create SQLite database: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("set SQLite database permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close SQLite database: %w", err)
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{Scheme: "file", Path: uriPath}
	dsn.RawQuery = url.Values{"_busy_timeout": {"5000"}, "_foreign_keys": {"on"}, "_journal_mode": {"WAL"}}.Encode()
	db, err := gorm.Open(sqlite.Open(dsn.String()), databaseGormConfig())
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	if err := configureDBPool(db, 1, 1); err != nil {
		return nil, err
	}

	return db, nil
}

func databaseGormConfig() *gorm.Config {
	return &gorm.Config{
		TranslateError: true,
		Logger: logger.New(log.New(os.Stderr, "database: ", log.LstdFlags), logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		}),
	}
}

func configureDBPool(db *gorm.DB, maxOpen, maxIdle int) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("access database connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return nil
}
