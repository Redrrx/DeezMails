package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const encryptedPrefix = "enc:v1:"

type Credentials struct {
	gcm cipher.AEAD
}

func newCredentials() *Credentials {
	encoded := env("CREDENTIALS_ENCRYPTION_KEY", "")
	if encoded == "" {
		panic("CREDENTIALS_ENCRYPTION_KEY is required")
	}
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err != nil || len(key) != 32 {
		panic("CREDENTIALS_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return &Credentials{gcm: gcm}
}

func (c *Credentials) encrypt(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nil, nonce, []byte(value), nil)
	return encryptedPrefix + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (c *Credentials) decrypt(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil || len(payload) < c.gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted credential")
	}
	plain, err := c.gcm.Open(nil, payload[:c.gcm.NonceSize()], payload[c.gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("could not decrypt credential: %w", err)
	}
	return string(plain), nil
}

func (a *App) sealAccount(account *Account) error {
	var err error
	if account.AccessToken, err = a.credentials.encrypt(account.AccessToken); err != nil {
		return err
	}
	if account.RefreshToken, err = a.credentials.encrypt(account.RefreshToken); err != nil {
		return err
	}
	account.MailboxPassword, err = a.credentials.encrypt(account.MailboxPassword)
	return err
}

func (a *App) openAccount(account *Account) error {
	var err error
	if account.AccessToken, err = a.credentials.decrypt(account.AccessToken); err != nil {
		return err
	}
	if account.RefreshToken, err = a.credentials.decrypt(account.RefreshToken); err != nil {
		return err
	}
	account.MailboxPassword, err = a.credentials.decrypt(account.MailboxPassword)
	if err != nil {
		return err
	}
	if account.Proxy != nil {
		return a.openProxy(account.Proxy)
	}
	return nil
}

func (a *App) sealProxy(proxy *Proxy) error {
	var err error
	proxy.Password, err = a.credentials.encrypt(proxy.Password)
	return err
}
func (a *App) openProxy(proxy *Proxy) error {
	var err error
	proxy.Password, err = a.credentials.decrypt(proxy.Password)
	return err
}

func (a *App) saveAccount(account *Account) error {
	stored := *account
	if err := a.sealAccount(&stored); err != nil {
		return err
	}
	return a.db.Save(&stored).Error
}

func (a *App) createAccountRecord(account *Account) error {
	stored := *account
	if err := a.sealAccount(&stored); err != nil {
		return err
	}
	if err := a.db.Create(&stored).Error; err != nil {
		return err
	}
	account.ID, account.CreatedAt, account.UpdatedAt = stored.ID, stored.CreatedAt, stored.UpdatedAt
	return nil
}

func (a *App) migrateCredentials() error {
	var accounts []Account
	if err := a.db.Find(&accounts).Error; err != nil {
		return err
	}
	for i := range accounts {
		account := &accounts[i]
		if needsEncryption(account.AccessToken) || needsEncryption(account.RefreshToken) || needsEncryption(account.MailboxPassword) {
			if err := a.saveAccount(account); err != nil {
				return err
			}
		}
	}
	var proxies []Proxy
	if err := a.db.Find(&proxies).Error; err != nil {
		return err
	}
	for i := range proxies {
		proxy := &proxies[i]
		changed := false
		if proxy.Host == "" && proxy.URL != "" {
			legacy, err := url.Parse(proxy.URL)
			if err != nil {
				return err
			}
			proxy.Type, proxy.Host = legacy.Scheme, legacy.Hostname()
			proxy.Port, _ = strconv.Atoi(legacy.Port())
			if legacy.User != nil {
				proxy.Username = legacy.User.Username()
				proxy.Password, _ = legacy.User.Password()
			}
			proxy.URL = (&url.URL{Scheme: proxy.Type, Host: legacy.Host}).String()
			changed = true
		}
		if changed || needsEncryption(proxy.Password) {
			if err := a.sealProxy(proxy); err != nil {
				return err
			}
			if err := a.db.Save(proxy).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func needsEncryption(value string) bool {
	return value != "" && !strings.HasPrefix(value, encryptedPrefix)
}
