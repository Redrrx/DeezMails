package storage

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"deezmails/internal/models"
)

func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DROP INDEX IF EXISTS account_remote").Error; err != nil {
			return fmt.Errorf("drop obsolete email index: %w", err)
		}
		if err := tx.AutoMigrate(&models.Proxy{}, &models.Account{}, &models.Email{}, &models.JobRun{}, &models.AppMetadata{}); err != nil {
			return fmt.Errorf("migrate database schema: %w", err)
		}
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS account_folder_remote ON emails (account_id, folder, remote_id)").Error; err != nil {
			return fmt.Errorf("create email identity index: %w", err)
		}
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS job_runs_active ON job_runs (account_id, type, folder) WHERE status IN ('queued', 'running', 'retrying')").Error; err != nil {
			return fmt.Errorf("create active-job index: %w", err)
		}
		if err := tx.Exec("CREATE INDEX IF NOT EXISTS emails_account_received ON emails (account_id, received_at DESC)").Error; err != nil {
			return fmt.Errorf("create email ordering index: %w", err)
		}
		if err := tx.Exec("CREATE INDEX IF NOT EXISTS emails_account_folder_received ON emails (account_id, folder, received_at DESC)").Error; err != nil {
			return fmt.Errorf("create folder ordering index: %w", err)
		}
		if err := tx.Exec("CREATE INDEX IF NOT EXISTS accounts_sync_due ON accounts (enabled, sync_enabled, status, next_sync_at)").Error; err != nil {
			return fmt.Errorf("create sync scheduler index: %w", err)
		}
		return nil
	})
}
