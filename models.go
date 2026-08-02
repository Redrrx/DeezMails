package main

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"gorm.io/gorm"
)

type Proxy struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	Type      string    `json:"type" gorm:"not null;default:http"`
	Host      string    `json:"host" gorm:"not null;default:''"`
	Port      int       `json:"port" gorm:"not null;default:0"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	URL       string    `json:"-" gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProxyInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (input ProxyInput) proxy() (Proxy, error) {
	if input.Name == "" || input.Host == "" || input.Port < 1 || input.Port > 65535 {
		return Proxy{}, fmt.Errorf("name, host, and a valid port are required")
	}
	if input.Type != "http" && input.Type != "https" && input.Type != "socks5" {
		return Proxy{}, fmt.Errorf("proxy type must be http, https, or socks5")
	}
	endpoint := &url.URL{Scheme: input.Type, Host: net.JoinHostPort(input.Host, fmt.Sprint(input.Port))}
	if input.Username != "" || input.Password != "" {
		endpoint.User = url.UserPassword(input.Username, input.Password)
	}
	return Proxy{Name: input.Name, Type: input.Type, Host: input.Host, Port: input.Port, Username: input.Username, Password: input.Password, URL: (&url.URL{Scheme: input.Type, Host: endpoint.Host}).String()}, nil
}

func (p Proxy) endpointURL() string {
	endpoint := &url.URL{Scheme: p.Type, Host: net.JoinHostPort(p.Host, fmt.Sprint(p.Port))}
	if p.Username != "" || p.Password != "" {
		endpoint.User = url.UserPassword(p.Username, p.Password)
	}
	return endpoint.String()
}

type Account struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	Email           string     `json:"email" gorm:"uniqueIndex;not null"`
	Provider        string     `json:"provider" gorm:"not null"`
	OAuthClientID   string     `json:"clientId" gorm:"not null;default:''"`
	IncomingHost    string     `json:"incomingHost" gorm:"not null;default:''"`
	IncomingPort    int        `json:"incomingPort" gorm:"not null;default:0"`
	TLSMode         string     `json:"tlsMode" gorm:"not null;default:implicit_tls"`
	Folder          string     `json:"folder" gorm:"not null;default:INBOX"`
	ProxyID         *uint      `json:"proxyId"`
	Proxy           *Proxy     `json:"proxy,omitempty"`
	Enabled         bool       `json:"enabled" gorm:"default:true"`
	SyncEnabled     bool       `json:"syncEnabled" gorm:"default:true"`
	Status          string     `json:"status" gorm:"default:disconnected"`
	LastSyncedAt    *time.Time `json:"lastSyncedAt"`
	LastCheckedAt   *time.Time `json:"lastCheckedAt"`
	NextSyncAt      *time.Time `json:"nextSyncAt" gorm:"index"`
	LastError       string     `json:"lastError"`
	AccessToken     string     `json:"-"`
	RefreshToken    string     `json:"-"`
	MailboxPassword string     `json:"-"`
	TokenExpiry     time.Time  `json:"-"`
	OAuthState      string     `json:"-"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type JobRun struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	QueueID       string     `json:"queueId" gorm:"index"`
	AccountID     *uint      `json:"accountId" gorm:"index"`
	Account       *Account   `json:"account,omitempty"`
	Type          string     `json:"type" gorm:"index;not null"`
	Folder        string     `json:"folder"`
	Status        string     `json:"status" gorm:"index;not null"`
	Attempts      int        `json:"attempts"`
	Fetched       int        `json:"fetched"`
	Created       int        `json:"created"`
	Updated       int        `json:"updated"`
	Synced        int        `json:"synced"`
	Error         string     `json:"error"`
	Logs          string     `json:"logs"`
	StartedAt     *time.Time `json:"startedAt"`
	NextAttemptAt *time.Time `json:"nextAttemptAt"`
	EndedAt       *time.Time `json:"endedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type AccountInput struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Provider     string `json:"provider"`
	ClientID     string `json:"clientId"`
	RefreshToken string `json:"refreshToken"`
	IncomingHost string `json:"incomingHost"`
	IncomingPort int    `json:"incomingPort"`
	TLSMode      string `json:"tlsMode"`
	Folder       string `json:"folder"`
	ProxyID      *uint  `json:"proxyId"`
}

// AccountUpdateInput uses pointers so an omitted field stays unchanged.
// Password and refreshToken only replace the saved credential when non-empty.
type AccountUpdateInput struct {
	Email        *string `json:"email"`
	Password     *string `json:"password"`
	Provider     *string `json:"provider"`
	ClientID     *string `json:"clientId"`
	RefreshToken *string `json:"refreshToken"`
	IncomingHost *string `json:"incomingHost"`
	IncomingPort *int    `json:"incomingPort"`
	TLSMode      *string `json:"tlsMode"`
	Folder       *string `json:"folder"`
	ProxyID      *uint   `json:"proxyId"`
	Enabled      *bool   `json:"enabled"`
}

type Email struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	AccountID  uint      `json:"accountId" gorm:"index;uniqueIndex:account_folder_remote;not null"`
	RemoteID   string    `json:"remoteId" gorm:"uniqueIndex:account_folder_remote;not null"`
	Folder     string    `json:"folder" gorm:"uniqueIndex:account_folder_remote;index;not null;default:INBOX"`
	ThreadID   string    `json:"threadId"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	Subject    string    `json:"subject"`
	Preview    string    `json:"preview"`
	Body       string    `json:"body"`
	Raw        string    `json:"-"`
	ReceivedAt time.Time `json:"receivedAt" gorm:"index"`
	IsRead     bool      `json:"isRead"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type App struct {
	db          *gorm.DB
	credentials *Credentials
	jobs        *JobService
	accessToken string
}
