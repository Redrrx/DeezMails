package models

import "time"

const (
	ProviderGmail     = "gmail"
	ProviderMicrosoft = "microsoft"
	ProviderIMAP      = "imap"
	ProviderPOP3      = "pop3"
	TLSModeImplicit   = "implicit_tls"
	TLSModeStartTLS   = "starttls"
	TLSModeNone       = "none"

	AccountStatusDisconnected      = "disconnected"
	AccountStatusConnected         = "connected"
	AccountStatusError             = "error"
	AccountStatusReconnectRequired = "reconnect_required"
)

type Account struct {
	ID                  uint            `json:"id" gorm:"primaryKey"`
	Email               string          `json:"email" gorm:"uniqueIndex;not null"`
	Provider            string          `json:"provider" gorm:"not null"`
	OAuthClientID       string          `json:"clientId" gorm:"not null;default:''"`
	IncomingHost        string          `json:"incomingHost" gorm:"not null;default:''"`
	IncomingPort        int             `json:"incomingPort" gorm:"not null;default:0"`
	TLSMode             string          `json:"tlsMode" gorm:"not null;default:implicit_tls"`
	Folder              string          `json:"folder" gorm:"not null;default:INBOX"`
	ProxyID             *uint           `json:"proxyId"`
	Proxy               *Proxy          `json:"proxy,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Enabled             bool            `json:"enabled" gorm:"not null"`
	SyncEnabled         bool            `json:"syncEnabled" gorm:"not null"`
	Status              string          `json:"status" gorm:"default:disconnected"`
	LastSyncedAt        *time.Time      `json:"lastSyncedAt"`
	LastCheckedAt       *time.Time      `json:"lastCheckedAt"`
	NextSyncAt          *time.Time      `json:"nextSyncAt" gorm:"index"`
	LastError           string          `json:"lastError"`
	LastErrorCode       MailErrorCode   `json:"lastErrorCode"`
	LastErrorAction     MailErrorAction `json:"lastErrorAction"`
	AccessToken         string          `json:"-"`
	RefreshToken        string          `json:"-"`
	MailboxPassword     string          `json:"-"`
	TokenExpiry         time.Time       `json:"-"`
	OAuthState          string          `json:"-"`
	OAuthCodeVerifier   string          `json:"-"`
	OAuthStateExpiresAt *time.Time      `json:"-"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}
