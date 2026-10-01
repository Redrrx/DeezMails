package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const (
	credentialPrefix     = "enc:v2:"
	credentialCheckKey   = "credential-key-check"
	credentialCheckValue = "deezmails-credential-key-check-v1"

	purposeAccountAccessToken  = "account.access-token"
	purposeAccountRefreshToken = "account.refresh-token"
	purposeAccountPassword     = "account.mailbox-password"
	purposeAccountPKCEVerifier = "account.pkce-verifier"
	purposeProxyPassword       = "proxy.password"
	purposeCredentialCheck     = "metadata.credential-key-check"
)

func newCredentialCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize credential AEAD: %w", err)
	}
	return gcm, nil
}

func encryptCredential(gcm cipher.AEAD, value, purpose string) (string, error) {
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(value), []byte(purpose))
	return credentialPrefix + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func decryptCredential(gcm cipher.AEAD, value, purpose string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, credentialPrefix) {
		return "", fmt.Errorf("credential is not encrypted with the current format")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, credentialPrefix))
	if err != nil || len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted credential")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], []byte(purpose))
	if err != nil {
		return "", fmt.Errorf("could not decrypt credential: %w", err)
	}
	return string(plain), nil
}
