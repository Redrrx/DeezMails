package app

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"deezmails/internal/models"
)

func (a *App) encryptAccountCredentials(account *models.Account) error {
	var err error
	if account.AccessToken, err = encryptCredential(a.credentials, account.AccessToken, purposeAccountAccessToken); err != nil {
		return err
	}
	if account.RefreshToken, err = encryptCredential(a.credentials, account.RefreshToken, purposeAccountRefreshToken); err != nil {
		return err
	}
	if account.OAuthCodeVerifier, err = encryptCredential(a.credentials, account.OAuthCodeVerifier, purposeAccountPKCEVerifier); err != nil {
		return err
	}
	account.MailboxPassword, err = encryptCredential(a.credentials, account.MailboxPassword, purposeAccountPassword)
	return err
}

func (a *App) decryptAccountCredentials(account *models.Account) error {
	var err error
	if account.AccessToken, err = decryptCredential(a.credentials, account.AccessToken, purposeAccountAccessToken); err != nil {
		return err
	}
	if account.RefreshToken, err = decryptCredential(a.credentials, account.RefreshToken, purposeAccountRefreshToken); err != nil {
		return err
	}
	if account.OAuthCodeVerifier, err = decryptCredential(a.credentials, account.OAuthCodeVerifier, purposeAccountPKCEVerifier); err != nil {
		return err
	}
	account.MailboxPassword, err = decryptCredential(a.credentials, account.MailboxPassword, purposeAccountPassword)
	if err != nil {
		return err
	}
	if account.Proxy != nil {
		account.Proxy.Password, err = decryptCredential(a.credentials, account.Proxy.Password, purposeProxyPassword)
	}
	return err
}

func (a *App) saveAccount(ctx context.Context, account *models.Account) error {
	stored := *account
	if err := a.encryptAccountCredentials(&stored); err != nil {
		return err
	}
	return a.db.WithContext(ctx).Omit(clause.Associations).Save(&stored).Error
}

func (a *App) createAccountRecord(ctx context.Context, account *models.Account) error {
	stored := *account
	if err := a.encryptAccountCredentials(&stored); err != nil {
		return err
	}
	if err := a.db.WithContext(ctx).Omit(clause.Associations).Create(&stored).Error; err != nil {
		return err
	}
	account.ID, account.CreatedAt, account.UpdatedAt = stored.ID, stored.CreatedAt, stored.UpdatedAt
	return nil
}

func (a *App) saveAccountTokens(ctx context.Context, account *models.Account) error {
	accessToken, err := encryptCredential(a.credentials, account.AccessToken, purposeAccountAccessToken)
	if err != nil {
		return err
	}
	refreshToken, err := encryptCredential(a.credentials, account.RefreshToken, purposeAccountRefreshToken)
	if err != nil {
		return err
	}
	return a.db.WithContext(ctx).Model(&models.Account{}).Where("id = ?", account.ID).Updates(map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_expiry":  account.TokenExpiry,
	}).Error
}

func (a *App) prepareCredentials(ctx context.Context) error {
	found, err := a.verifyCredentialKeyCheck(ctx)
	if err != nil {
		return err
	}
	if !found {
		return a.createCredentialKeyCheck(ctx)
	}
	return nil
}

func (a *App) verifyCredentialKeyCheck(ctx context.Context) (bool, error) {
	var metadata models.AppMetadata
	err := a.db.WithContext(ctx).First(&metadata, "key = ?", credentialCheckKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load credential-key check: %w", err)
	}
	plain, err := decryptCredential(a.credentials, metadata.Value, purposeCredentialCheck)
	if err != nil || subtle.ConstantTimeCompare([]byte(plain), []byte(credentialCheckValue)) != 1 {
		return true, fmt.Errorf("the configured credential encryption key does not match this database")
	}
	return true, nil
}

func (a *App) createCredentialKeyCheck(ctx context.Context) error {
	value, err := encryptCredential(a.credentials, credentialCheckValue, purposeCredentialCheck)
	if err != nil {
		return err
	}
	metadata := models.AppMetadata{Key: credentialCheckKey, Value: value}
	if err := a.db.WithContext(ctx).Create(&metadata).Error; err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("save credential-key check: %w", err)
		}
		_, err = a.verifyCredentialKeyCheck(ctx)
		return err
	}
	return nil
}
