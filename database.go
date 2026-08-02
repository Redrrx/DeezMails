package main

import (
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDB() *gorm.DB {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		configureDBPool(db, 20, 10)
		return db
	}

	db, err := gorm.Open(sqlite.Open("file:deezmails.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	configureDBPool(db, 1, 1)
	return db
}

func configureDBPool(db *gorm.DB, maxOpen, maxIdle int) {
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
}
