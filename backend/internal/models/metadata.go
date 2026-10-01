package models

import "time"

type AppMetadata struct {
	Key       string `gorm:"primaryKey;size:100"`
	Value     string `gorm:"not null"`
	UpdatedAt time.Time
}
