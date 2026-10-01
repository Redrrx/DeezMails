package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"deezmails/internal/models"
)

func (a *App) syncAccount(ctx context.Context, id uint, folder string) (syncResult, error) {
	result := syncResult{}
	var account models.Account
	db := a.db.WithContext(ctx)
	if err := db.Preload("Proxy").First(&account, id).Error; err != nil {
		return result, err
	}
	if err := a.decryptAccountCredentials(&account); err != nil {
		return result, err
	}
	if err := a.validateMailTransport(account); err != nil {
		return result, err
	}
	if !account.Enabled || !account.SyncEnabled || account.Status == models.AccountStatusDisconnected || account.Status == models.AccountStatusReconnectRequired {
		return result, fmt.Errorf("account is not ready to sync")
	}
	var emails []models.Email
	var err error
	if account.Provider == models.ProviderIMAP || account.Provider == models.ProviderPOP3 {
		if folder != folderAll && !strings.EqualFold(folder, "INBOX") {
			if account.Provider == models.ProviderPOP3 {
				return result, fmt.Errorf("POP3 exposes only INBOX")
			}
		}
		if account.Provider == models.ProviderIMAP {
			emails, err = a.imapMessages(ctx, account, folder)
		} else {
			emails, err = a.pop3Messages(ctx, account)
		}
	} else {
		token, tokenErr := a.token(ctx, &account)
		if tokenErr != nil {
			failure := setAccountFailure(&account, tokenErr)
			if err := a.saveAccount(ctx, &account); err != nil {
				return result, err
			}
			return result, failure
		}
		if account.Provider == models.ProviderGmail {
			emails, err = a.gmailMessages(ctx, account, token, folder)
		} else {
			emails, err = a.microsoftMessages(ctx, account, token, folder)
		}
	}
	checked := time.Now()
	account.LastCheckedAt = &checked
	if err != nil {
		failure := setAccountFailure(&account, err)
		if saveErr := a.saveAccount(ctx, &account); saveErr != nil {
			return result, saveErr
		}
		return result, failure
	}
	result.Fetched = len(emails)
	result.Created, result.Updated, err = a.persistEmails(ctx, emails)
	if err != nil {
		failure := setAccountFailure(&account, err)
		if saveErr := a.saveAccount(ctx, &account); saveErr != nil {
			return result, saveErr
		}
		return result, failure
	}
	next := checked.Add(a.config.SyncInterval)
	account.LastSyncedAt, account.NextSyncAt, account.LastError, account.Status = &checked, &next, "", models.AccountStatusConnected
	account.LastErrorCode, account.LastErrorAction = "", ""
	if err := a.saveAccount(ctx, &account); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) persistEmails(ctx context.Context, emails []models.Email) (int, int, error) {
	db := a.db.WithContext(ctx)
	byFolder := map[string][]models.Email{}
	for _, email := range emails {
		byFolder[email.Folder] = append(byFolder[email.Folder], email)
	}
	created, updated := 0, 0
	for folder, items := range byFolder {
		for start := 0; start < len(items); start += 100 {
			end := start + 100
			if end > len(items) {
				end = len(items)
			}
			batch := items[start:end]
			remoteIDs := make([]string, 0, len(batch))
			for _, email := range batch {
				remoteIDs = append(remoteIDs, email.RemoteID)
			}
			var existing []models.Email
			if err := db.Select("remote_id").Where("account_id = ? AND folder = ? AND remote_id IN ?", batch[0].AccountID, folder, remoteIDs).Find(&existing).Error; err != nil {
				return created, updated, err
			}
			existingIDs := map[string]bool{}
			for _, email := range existing {
				existingIDs[email.RemoteID] = true
			}
			for _, email := range batch {
				if existingIDs[email.RemoteID] {
					updated++
				} else {
					created++
				}
			}
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "account_id"}, {Name: "folder"}, {Name: "remote_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"thread_id", "from", "to", "subject", "preview", "received_at", "is_read"}),
			}).Create(&batch).Error; err != nil {
				return created, updated, err
			}
		}
	}
	return created, updated, nil
}

func (a *App) verifyAccount(ctx context.Context, id uint) error {
	var account models.Account
	if err := a.db.WithContext(ctx).Preload("Proxy").First(&account, id).Error; err != nil {
		return err
	}
	if err := a.decryptAccountCredentials(&account); err != nil {
		return err
	}
	if err := a.validateMailTransport(account); err != nil {
		return err
	}
	if !account.Enabled {
		return fmt.Errorf("account is disabled")
	}
	var err error
	if account.Provider == models.ProviderIMAP {
		_, err = a.imapFolders(ctx, account)
	} else if account.Provider == models.ProviderPOP3 {
		err = pop3Verify(ctx, account)
	} else {
		_, err = a.token(ctx, &account)
	}
	checked := time.Now()
	account.LastCheckedAt = &checked
	if err != nil {
		failure := setAccountFailure(&account, err)
		if saveErr := a.saveAccount(ctx, &account); saveErr != nil {
			return saveErr
		}
		return failure
	}
	account.Status, account.LastError, account.LastErrorCode, account.LastErrorAction = models.AccountStatusConnected, "", "", ""
	if err := a.saveAccount(ctx, &account); err != nil {
		return err
	}
	if account.Enabled && account.SyncEnabled {
		if _, err := a.enqueueJob(ctx, models.JobTypeSyncAccount, account.ID, folderAll); err != nil {
			return err
		}
	}
	return nil
}
