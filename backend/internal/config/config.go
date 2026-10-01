package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"deezmails/internal/validation"

	envconfig "github.com/caarlos0/env/v11"
)

type Config struct {
	ListenAddress         string
	DatabaseURL           string
	SQLitePath            string
	FrontendDistDir       string
	CredentialKey         []byte
	AccessToken           string
	OAuthRedirectURL      string
	AllowInsecureMailAuth bool
	GmailClientSecret     string
	MicrosoftClientSecret string
	WorkerConcurrency     int
	SchedulerBatchSize    int
	SyncInterval          time.Duration
	SyncMaxPages          int
	SyncMaxMessages       int
	SyncMaxFolders        int
	JobTimeout            time.Duration
	JobMaxAttempts        int
	JobRetryBase          time.Duration
	MaxMessageBytes       int64
}

type environment struct {
	Host                  string `env:"HOST"`
	Port                  int    `env:"PORT" envDefault:"8080"`
	DatabaseURL           string `env:"DATABASE_URL"`
	SQLitePath            string `env:"SQLITE_PATH"`
	FrontendDistDir       string `env:"FRONTEND_DIST_DIR"`
	CredentialKey         string `env:"CREDENTIALS_ENCRYPTION_KEY"`
	AccessToken           string `env:"DEEZMAILS_ACCESS_TOKEN"`
	OAuthRedirectURL      string `env:"OAUTH_REDIRECT_URL"`
	GmailClientSecret     string `env:"GMAIL_CLIENT_SECRET"`
	MicrosoftClientSecret string `env:"MICROSOFT_CLIENT_SECRET"`
	AllowInsecureMailAuth bool   `env:"DEEZMAILS_ALLOW_INSECURE_MAIL" envDefault:"false"`
	WorkerConcurrency     int    `env:"WORKER_CONCURRENCY"`
	SchedulerBatchSize    int    `env:"SCHEDULER_BATCH_SIZE" envDefault:"50"`
	SyncIntervalMinutes   int    `env:"SYNC_INTERVAL_MINUTES" envDefault:"15"`
	SyncMaxPages          int    `env:"SYNC_MAX_PAGES" envDefault:"5"`
	SyncMaxMessages       int    `env:"SYNC_MAX_MESSAGES" envDefault:"500"`
	SyncMaxFolders        int    `env:"SYNC_MAX_FOLDERS" envDefault:"500"`
	JobTimeoutMinutes     int    `env:"JOB_TIMEOUT_MINUTES" envDefault:"10"`
	JobMaxAttempts        int    `env:"JOB_MAX_ATTEMPTS" envDefault:"3"`
	JobRetryBaseSeconds   int    `env:"JOB_RETRY_BASE_SECONDS" envDefault:"15"`
	MaxMessageBytes       int64  `env:"MAX_MESSAGE_BYTES" envDefault:"67108864"`
}

func Load() (Config, error) {
	variables, err := loadEnvironment(".env", "backend/.env")
	if err != nil {
		return Config{}, err
	}
	values, err := envconfig.ParseAsWithOptions[environment](envconfig.Options{Environment: variables})
	if err != nil {
		return Config{}, fmt.Errorf("environment: %w", err)
	}
	config := Config{}

	if values.Port < 1 || values.Port > 65535 {
		return config, fmt.Errorf("PORT must be between 1 and 65535")
	}
	host := validation.NormalizeNetworkHost(strings.TrimSpace(values.Host))
	if host == "" {
		host = "0.0.0.0"
	}
	if host == "" || strings.ContainsAny(host, "/\x00\r\n") {
		return config, fmt.Errorf("HOST is invalid")
	}
	config.ListenAddress = net.JoinHostPort(host, strconv.Itoa(values.Port))

	config.DatabaseURL = strings.TrimSpace(values.DatabaseURL)
	config.SQLitePath = strings.TrimSpace(values.SQLitePath)
	if config.SQLitePath == "" {
		config.SQLitePath = "deezmails.db"
	}

	encodedKey := strings.TrimSpace(values.CredentialKey)
	if encodedKey == "" {
		return config, fmt.Errorf("CREDENTIALS_ENCRYPTION_KEY is required")
	}
	config.CredentialKey, err = base64.RawStdEncoding.DecodeString(encodedKey)
	if err != nil {
		config.CredentialKey, err = base64.StdEncoding.DecodeString(encodedKey)
	}
	if err != nil || len(config.CredentialKey) != 32 {
		return config, fmt.Errorf("CREDENTIALS_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}

	config.AccessToken = values.AccessToken
	if config.AccessToken != "" && (len(config.AccessToken) < 32 || len(config.AccessToken) > 4096 || validation.HasLineBreakOrNUL(config.AccessToken)) {
		return config, fmt.Errorf("DEEZMAILS_ACCESS_TOKEN must contain 32 to 4096 characters without line breaks")
	}
	if config.AccessToken == "" {
		return config, fmt.Errorf("DEEZMAILS_ACCESS_TOKEN is required")
	}

	redirectFallback := "http://localhost:" + strconv.Itoa(values.Port) + "/oauth/callback"
	config.OAuthRedirectURL = strings.TrimSpace(values.OAuthRedirectURL)
	if config.OAuthRedirectURL == "" {
		config.OAuthRedirectURL = redirectFallback
	}
	if err := validateOAuthRedirect(config.OAuthRedirectURL); err != nil {
		return config, err
	}

	workers := values.WorkerConcurrency
	if workers == 0 {
		workers = 1
		if config.DatabaseURL != "" {
			workers = 4
		}
	}
	if workers < 1 || workers > 32 {
		return config, fmt.Errorf("WORKER_CONCURRENCY must be between 1 and 32")
	}
	if values.SchedulerBatchSize < 1 || values.SchedulerBatchSize > 1000 {
		return config, fmt.Errorf("SCHEDULER_BATCH_SIZE must be between 1 and 1000")
	}
	if values.SyncIntervalMinutes < 1 || values.SyncIntervalMinutes > 7*24*60 {
		return config, fmt.Errorf("SYNC_INTERVAL_MINUTES must be between 1 and 10080")
	}
	if values.SyncMaxPages < 1 || values.SyncMaxPages > 50 {
		return config, fmt.Errorf("SYNC_MAX_PAGES must be between 1 and 50")
	}
	if values.SyncMaxMessages < 1 || values.SyncMaxMessages > 5000 {
		return config, fmt.Errorf("SYNC_MAX_MESSAGES must be between 1 and 5000")
	}
	if values.SyncMaxFolders < 1 || values.SyncMaxFolders > 2000 {
		return config, fmt.Errorf("SYNC_MAX_FOLDERS must be between 1 and 2000")
	}
	if values.JobTimeoutMinutes < 1 || values.JobTimeoutMinutes > 24*60 {
		return config, fmt.Errorf("JOB_TIMEOUT_MINUTES must be between 1 and 1440")
	}
	if values.JobMaxAttempts < 1 || values.JobMaxAttempts > 10 {
		return config, fmt.Errorf("JOB_MAX_ATTEMPTS must be between 1 and 10")
	}
	if values.JobRetryBaseSeconds < 1 || values.JobRetryBaseSeconds > 300 {
		return config, fmt.Errorf("JOB_RETRY_BASE_SECONDS must be between 1 and 300")
	}
	if values.MaxMessageBytes < 1 || values.MaxMessageBytes > 512<<20 {
		return config, fmt.Errorf("MAX_MESSAGE_BYTES must be between 1 and 536870912")
	}

	config.AllowInsecureMailAuth = values.AllowInsecureMailAuth
	config.GmailClientSecret = values.GmailClientSecret
	config.MicrosoftClientSecret = values.MicrosoftClientSecret
	config.WorkerConcurrency = workers
	config.SchedulerBatchSize = values.SchedulerBatchSize
	config.SyncInterval = time.Duration(values.SyncIntervalMinutes) * time.Minute
	config.SyncMaxPages = values.SyncMaxPages
	config.SyncMaxMessages = values.SyncMaxMessages
	config.SyncMaxFolders = values.SyncMaxFolders
	config.JobTimeout = time.Duration(values.JobTimeoutMinutes) * time.Minute
	config.JobMaxAttempts = values.JobMaxAttempts
	config.JobRetryBase = time.Duration(values.JobRetryBaseSeconds) * time.Second
	config.MaxMessageBytes = values.MaxMessageBytes
	config.FrontendDistDir, err = getFrontendDistDir(values.FrontendDistDir)
	if err != nil {
		return config, err
	}
	return config, nil
}

func validateOAuthRedirect(value string) error {
	redirect, err := url.Parse(value)
	if err != nil || !redirect.IsAbs() || redirect.Host == "" || (redirect.Scheme != "http" && redirect.Scheme != "https") {
		return fmt.Errorf("OAUTH_REDIRECT_URL must be an absolute http or https URL")
	}
	if redirect.Scheme != "https" && !validation.IsLoopbackHost(redirect.Hostname()) {
		return fmt.Errorf("OAUTH_REDIRECT_URL must use https unless it targets loopback")
	}
	return nil
}
