package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "deezmails/docs"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// @title DeezMails API
// @version 0.1.0
// @description Manage OAuth-connected Gmail and Microsoft mailboxes, proxies, and synced messages.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
//
//go:generate go run github.com/swaggo/swag/cmd/swag init -g main.go
func main() {
	db := openDB()
	if err := db.Exec("DROP INDEX IF EXISTS account_remote").Error; err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&Proxy{}, &Account{}, &Email{}, &JobRun{}); err != nil {
		panic(err)
	}
	if err := db.Exec("DROP INDEX IF EXISTS account_folder_remote").Error; err != nil {
		panic(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS account_folder_remote ON emails (account_id, folder, remote_id)").Error; err != nil {
		panic(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS job_runs_active ON job_runs (account_id, type, folder) WHERE status IN ('queued', 'running', 'retrying')").Error; err != nil {
		panic(err)
	}
	app := &App{db: db, credentials: newCredentials(), accessToken: newAccessToken()}
	if err := app.migrateCredentials(); err != nil {
		panic(err)
	}
	if err := app.startJobs(); err != nil {
		panic(err)
	}
	r := gin.Default()
	r.Use(securityHeaders())
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	routes := r.Group("/api", app.requireAccessToken())
	routes.Use(limitRequestBody(1 << 20))
	routes.POST("/proxies", app.createProxy)
	routes.GET("/proxies", app.listProxies)
	routes.PUT("/proxies/:id", app.updateProxy)
	routes.DELETE("/proxies/:id", app.deleteProxy)
	routes.POST("/accounts", app.createAccount)
	routes.GET("/accounts", app.listAccounts)
	routes.GET("/accounts/:id", app.getAccount)
	routes.PUT("/accounts/:id", app.updateAccount)
	routes.DELETE("/accounts/:id", app.deleteAccount)
	routes.POST("/accounts/:id/reconnect", app.reconnect)
	r.GET("/oauth/callback", app.oauthCallback)
	routes.POST("/accounts/:id/sync", app.sync)
	routes.GET("/accounts/:id/folders", app.listFolders)
	routes.GET("/accounts/:id/emails", app.listEmails)
	routes.GET("/accounts/:id/emails/:messageID", app.getEmail)
	routes.GET("/jobs", app.listJobs)
	routes.DELETE("/jobs", app.clearJobs)
	routes.GET("/jobs/:id", app.getJob)
	routes.POST("/jobs/:id/retry", app.retryJob)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	if _, err := os.Stat("dist/index.html"); err == nil {
		r.Static("/assets", "./dist/assets")
		r.GET("/", func(c *gin.Context) { c.File("./dist/index.html") })
	}
	port := env("PORT", "8080")
	slog.Info("server started", "port", port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

// createProxy godoc
// @Summary Create a proxy
// @Tags proxies
// @Accept json
// @Produce json
// @Param proxy body ProxyInput true "Proxy"
// @Success 201 {object} Proxy
// @Failure 400 {object} map[string]string
// @Router /api/proxies [post]
func (a *App) createProxy(c *gin.Context) {
	var input ProxyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		bad(c, "invalid proxy input")
		return
	}
	p, err := input.proxy()
	if err != nil {
		bad(c, err.Error())
		return
	}
	if err := a.sealProxy(&p); err != nil {
		bad(c, err.Error())
		return
	}
	if err := a.db.Create(&p).Error; err != nil {
		bad(c, err.Error())
		return
	}
	c.JSON(http.StatusCreated, p)
}

// listProxies godoc
// @Summary List proxies
// @Tags proxies
// @Produce json
// @Success 200 {array} Proxy
// @Router /api/proxies [get]
func (a *App) listProxies(c *gin.Context) {
	var items []Proxy
	if err := a.db.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load proxies"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// updateProxy godoc
// @Summary Update a proxy
// @Tags proxies
// @Accept json
// @Produce json
// @Param id path int true "Proxy ID"
// @Param proxy body ProxyInput true "Proxy"
// @Success 200 {object} Proxy
// @Router /api/proxies/{id} [put]
func (a *App) updateProxy(c *gin.Context) {
	var p Proxy
	if a.db.First(&p, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	var input ProxyInput
	if c.ShouldBindJSON(&input) != nil {
		bad(c, "invalid proxy input")
		return
	}
	next, err := input.proxy()
	if err != nil {
		bad(c, err.Error())
		return
	}
	p.Name, p.Type, p.Host, p.Port, p.Username, p.URL = next.Name, next.Type, next.Host, next.Port, next.Username, next.URL
	if next.Password != "" {
		p.Password = next.Password
		if err := a.sealProxy(&p); err != nil {
			bad(c, err.Error())
			return
		}
	}
	if err := a.db.Save(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update proxy"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// deleteProxy godoc
// @Summary Delete a proxy
// @Tags proxies
// @Param id path int true "Proxy ID"
// @Success 204
// @Router /api/proxies/{id} [delete]
func (a *App) deleteProxy(c *gin.Context) {
	var p Proxy
	if a.db.First(&p, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Account{}).Where("proxy_id = ?", p.ID).Update("proxy_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete proxy"})
		return
	}
	c.Status(http.StatusNoContent)
}

// createAccount godoc
// @Summary Create mailbox account metadata
// @Tags accounts
// @Accept json
// @Produce json
// @Param account body AccountInput true "Account"
// @Success 201 {object} Account
// @Router /api/accounts [post]
func (a *App) createAccount(c *gin.Context) {
	var input AccountInput
	if c.ShouldBindJSON(&input) != nil || input.Email == "" || !validProvider(input.Provider) || (usesOAuth(input.Provider) && input.ClientID == "") || !validIncoming(input) {
		bad(c, "invalid account configuration")
		return
	}
	account := Account{Email: input.Email, Provider: input.Provider, OAuthClientID: input.ClientID, MailboxPassword: input.Password, RefreshToken: input.RefreshToken, IncomingHost: input.IncomingHost, IncomingPort: input.IncomingPort, TLSMode: input.TLSMode, Folder: input.Folder, ProxyID: input.ProxyID}
	account.Enabled, account.SyncEnabled = true, true
	account.Status = "disconnected"
	if account.RefreshToken != "" {
		account.Status = "connected"
	}
	if account.ProxyID != nil && a.db.First(&Proxy{}, *account.ProxyID).Error != nil {
		bad(c, "proxy not found")
		return
	}
	if err := a.createAccountRecord(&account); err != nil {
		bad(c, err.Error())
		return
	}
	if !usesOAuth(account.Provider) || account.RefreshToken != "" {
		if _, err := a.enqueueJob(jobVerify, account.ID, ""); err != nil {
			bad(c, err.Error())
			return
		}
	}
	c.JSON(http.StatusCreated, account)
}

// listAccounts godoc
// @Summary List mailbox accounts
// @Tags accounts
// @Produce json
// @Param provider query string false "Filter provider" Enums(gmail,microsoft)
// @Success 200 {array} Account
// @Router /api/accounts [get]
func (a *App) listAccounts(c *gin.Context) {
	var items []Account
	q := a.db.Preload("Proxy").Order("id desc")
	if provider := c.Query("provider"); provider != "" {
		q = q.Where("provider = ?", provider)
	}
	if err := q.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load accounts"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// getAccount godoc
// @Summary Get account
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {object} Account
// @Router /api/accounts/{id} [get]
func (a *App) getAccount(c *gin.Context) {
	var account Account
	if a.db.Preload("Proxy").First(&account, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, account)
}

// updateAccount godoc
// @Summary Update an account
// @Tags accounts
// @Accept json
// @Produce json
// @Param id path int true "Account ID"
// @Param account body AccountUpdateInput true "Account update"
// @Success 200 {object} Account
// @Router /api/accounts/{id} [put]
func (a *App) updateAccount(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	var in AccountUpdateInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c, "invalid json")
		return
	}

	providerChanged := false
	if in.Provider != nil {
		provider := strings.TrimSpace(*in.Provider)
		if !validProvider(provider) {
			bad(c, "invalid provider")
			return
		}
		providerChanged = provider != account.Provider
		account.Provider = provider
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if email == "" {
			bad(c, "email is required")
			return
		}
		account.Email = email
	}
	if in.Enabled != nil {
		account.Enabled = *in.Enabled
	}
	if in.ProxyID != nil {
		if *in.ProxyID == 0 {
			account.ProxyID, account.Proxy = nil, nil
		} else {
			var proxy Proxy
			if a.db.First(&proxy, *in.ProxyID).Error != nil {
				bad(c, "proxy not found")
				return
			}
			proxyID := proxy.ID
			account.ProxyID, account.Proxy = &proxyID, &proxy
		}
	}
	if in.Folder != nil {
		account.Folder = strings.TrimSpace(*in.Folder)
	}

	connectionChanged := providerChanged || in.ClientID != nil || in.RefreshToken != nil || in.Password != nil || in.IncomingHost != nil || in.IncomingPort != nil || in.TLSMode != nil || in.ProxyID != nil
	if usesOAuth(account.Provider) {
		if in.ClientID != nil {
			account.OAuthClientID = strings.TrimSpace(*in.ClientID)
		}
		if account.OAuthClientID == "" {
			bad(c, "OAuth client ID is required")
			return
		}
		if in.RefreshToken != nil && strings.TrimSpace(*in.RefreshToken) != "" {
			account.RefreshToken = strings.TrimSpace(*in.RefreshToken)
		}
		if providerChanged || in.ClientID != nil || in.RefreshToken != nil {
			account.AccessToken, account.OAuthState = "", ""
			account.TokenExpiry = time.Time{}
		}
		account.MailboxPassword = ""
		account.IncomingHost, account.IncomingPort = "", 0
		account.TLSMode = "implicit_tls"
	} else {
		if providerChanged {
			account.RefreshToken = ""
		}
		account.OAuthClientID, account.AccessToken, account.OAuthState = "", "", ""
		account.TokenExpiry = time.Time{}
		if in.Password != nil && *in.Password != "" {
			account.MailboxPassword = *in.Password
		}
		if in.IncomingHost != nil {
			account.IncomingHost = strings.TrimSpace(*in.IncomingHost)
		}
		if in.IncomingPort != nil {
			account.IncomingPort = *in.IncomingPort
		}
		if in.TLSMode != nil {
			account.TLSMode = *in.TLSMode
		}
		if !validIncoming(AccountInput{Provider: account.Provider, Password: account.MailboxPassword, IncomingHost: account.IncomingHost, IncomingPort: account.IncomingPort, TLSMode: account.TLSMode}) {
			bad(c, "a host, valid port, security mode, and password are required for IMAP/POP3")
			return
		}
	}

	needsVerification := !usesOAuth(account.Provider) || account.RefreshToken != ""
	if connectionChanged {
		account.Status, account.LastError = "disconnected", ""
	}
	if err := a.saveAccount(&account); err != nil {
		bad(c, "could not save account; the email may already exist")
		return
	}
	if connectionChanged && needsVerification {
		if _, err := a.enqueueJob(jobVerify, account.ID, ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "account saved but connection check could not be queued"})
			return
		}
	}
	c.JSON(http.StatusOK, account)
}

// deleteAccount godoc
// @Summary Delete account and synced messages
// @Tags accounts
// @Param id path int true "Account ID"
// @Success 204
// @Router /api/accounts/{id} [delete]
func (a *App) deleteAccount(c *gin.Context) {
	var account Account
	if a.db.First(&account, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	var running int64
	if err := a.db.Model(&JobRun{}).Where("account_id = ? AND status = ?", account.ID, "running").Count(&running).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check account jobs"})
		return
	}
	if running > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "account has a running job; wait for it to finish, then remove it"})
		return
	}
	if err := a.removeQueuedJobs(account.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", account.ID).Delete(&JobRun{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", account.ID).Delete(&Email{}).Error; err != nil {
			return err
		}
		return tx.Delete(&account).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// reconnect godoc
// @Summary Reconnect a mailbox
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {object} map[string]string
// @Success 202 {object} JobRun
// @Router /api/accounts/{id}/reconnect [post]
func (a *App) reconnect(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	if !account.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "account is disabled"})
		return
	}
	if account.Status == "connected" {
		c.JSON(http.StatusConflict, gin.H{"error": "account is already connected; sync it instead"})
		return
	}
	if !usesOAuth(account.Provider) {
		job, err := a.enqueueJob(jobVerify, account.ID, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, job)
		return
	}
	if account.OAuthState != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "OAuth approval is already in progress"})
		return
	}
	state := randomHex(24)
	account.OAuthState = state
	if err := a.saveAccount(&account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not prepare OAuth connection"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": oauthConfig(account.Provider, account.OAuthClientID).AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)})
}

// oauthCallback godoc
// @Summary OAuth provider callback
// @Tags OAuth
// @Param state query string true "OAuth state"
// @Param code query string true "Authorization code"
// @Success 200 {string} string "Account connected"
// @Router /oauth/callback [get]
func (a *App) oauthCallback(c *gin.Context) {
	state, code := c.Query("state"), c.Query("code")
	var account Account
	if state == "" || code == "" || a.db.Where("oauth_state = ?", state).First(&account).Error != nil {
		c.String(http.StatusBadRequest, "invalid OAuth callback")
		return
	}
	if err := a.openAccount(&account); err != nil {
		c.String(http.StatusInternalServerError, "credential decryption failed")
		return
	}
	token, err := oauthConfig(account.Provider, account.OAuthClientID).Exchange(c.Request.Context(), code)
	if err != nil {
		account.Status = "error"
		if saveErr := a.saveAccount(&account); saveErr != nil {
			c.String(http.StatusInternalServerError, "could not save OAuth failure")
			return
		}
		c.String(http.StatusBadRequest, "token exchange failed: %s", err)
		return
	}
	account.AccessToken, account.RefreshToken, account.TokenExpiry, account.OAuthState, account.Status = token.AccessToken, token.RefreshToken, token.Expiry, "", "connected"
	if err := a.saveAccount(&account); err != nil {
		c.String(http.StatusInternalServerError, "could not save OAuth token")
		return
	}
	if account.Enabled && account.SyncEnabled {
		if _, err := a.enqueueJob(jobSync, account.ID, "all"); err != nil {
			c.String(http.StatusInternalServerError, "connected, but could not queue the initial sync")
			return
		}
	}
	c.String(http.StatusOK, "Connected %s. You may close this page.", account.Email)
}

// sync godoc
// @Summary Queue a mailbox sync job
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Param folder query string false "Folder ID or all" default(all)
// @Success 202 {object} JobRun
// @Router /api/accounts/{id}/sync [post]
func (a *App) sync(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	if !account.Enabled || !account.SyncEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "account syncing is disabled"})
		return
	}
	if account.Status != "connected" {
		c.JSON(http.StatusConflict, gin.H{"error": "account is not connected; reconnect it first"})
		return
	}
	job, err := a.enqueueJob(jobSync, account.ID, c.DefaultQuery("folder", "all"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// listJobs godoc
// @Summary List durable job runs
// @Tags jobs
// @Produce json
// @Param status query string false "Filter status"
// @Param accountId query int false "Filter account"
// @Success 200 {array} JobRun
// @Router /api/jobs [get]
func (a *App) listJobs(c *gin.Context) {
	query := a.db.Preload("Account").Order("id desc").Limit(200)
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if accountID := c.Query("accountId"); accountID != "" {
		query = query.Where("account_id = ?", accountID)
	}
	var jobs []JobRun
	if err := query.Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load jobs"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// clearJobs godoc
// @Summary Clear completed and failed job history
// @Tags jobs
// @Produce json
// @Success 200 {object} map[string]int
// @Router /api/jobs [delete]
func (a *App) clearJobs(c *gin.Context) {
	result := a.db.Where("status IN ?", []string{"succeeded", "failed"}).Delete(&JobRun{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not clear job history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

// getJob godoc
// @Summary Get a durable job run
// @Tags jobs
// @Produce json
// @Param id path int true "Job ID"
// @Success 200 {object} JobRun
// @Router /api/jobs/{id} [get]
func (a *App) getJob(c *gin.Context) {
	var job JobRun
	if a.db.Preload("Account").First(&job, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, job)
}

// retryJob godoc
// @Summary Retry a failed job
// @Tags jobs
// @Produce json
// @Param id path int true "Job ID"
// @Success 202 {object} JobRun
// @Router /api/jobs/{id}/retry [post]
func (a *App) retryJob(c *gin.Context) {
	var previous JobRun
	if a.db.First(&previous, c.Param("id")).Error != nil {
		notFound(c)
		return
	}
	if previous.AccountID == nil {
		bad(c, "job has no account")
		return
	}
	job, err := a.enqueueJob(previous.Type, *previous.AccountID, previous.Folder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// listEmails godoc
// @Summary List synced emails
// @Tags emails
// @Produce json
// @Param id path int true "Account ID"
// @Param page query int false "Page" default(1)
// @Param pageSize query int false "Page size" default(50)
// @Param q query string false "Search sender, subject, and preview"
// @Param folder query string false "Folder ID or name; defaults to all folders" default(all)
// @Param sort query string false "Sort field" Enums(receivedAt,subject,from,createdAt)
// @Param direction query string false "Sort direction" Enums(asc,desc)
// @Success 200 {object} map[string]interface{}
// @Router /api/accounts/{id}/emails [get]
func (a *App) listEmails(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	page := positive(c.Query("page"), 1)
	size := positive(c.Query("pageSize"), 50)
	if size > 100 {
		size = 100
	}
	sort := c.DefaultQuery("sort", "receivedAt")
	direction := strings.ToLower(c.DefaultQuery("direction", "desc"))
	allowed := map[string]string{"receivedAt": "received_at", "subject": "subject", "from": "\"from\"", "createdAt": "created_at"}
	column, exists := allowed[sort]
	if !exists || (direction != "asc" && direction != "desc") {
		bad(c, "invalid sort or direction")
		return
	}
	var total int64
	q := a.db.Model(&Email{}).Where("account_id = ?", account.ID)
	if folder := c.DefaultQuery("folder", "all"); folder != "all" {
		q = q.Where("folder = ?", folder)
	}
	if text := c.Query("q"); text != "" {
		q = q.Where("subject LIKE ? OR \"from\" LIKE ? OR preview LIKE ?", "%"+text+"%", "%"+text+"%", "%"+text+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count emails"})
		return
	}
	var emails []Email
	if err := q.Order(column + " " + direction).Limit(size).Offset((page - 1) * size).Find(&emails).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load emails"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": emails, "page": page, "pageSize": size, "total": total})
}

// listFolders godoc
// @Summary List available mailbox folders
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {array} Folder
// @Router /api/accounts/{id}/folders [get]
func (a *App) listFolders(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	if account.Provider == "pop3" {
		c.JSON(http.StatusOK, []Folder{{ID: "INBOX", Name: "Inbox"}})
		return
	}
	if account.Provider == "imap" {
		folders, err := imapFolders(account)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, folders)
		return
	}
	token, err := a.token(c.Request.Context(), &account)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth token could not be refreshed; reconnect the account"})
		return
	}
	if account.Provider == "gmail" {
		folders, err := gmailFolders(c.Request.Context(), account, token)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, folders)
		return
	}
	folders, err := microsoftFolders(c.Request.Context(), account, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, folders)
}

// getEmail godoc
// @Summary Get an email in JSON or raw RFC 822 format
// @Tags emails
// @Produce json
// @Param id path int true "Account ID"
// @Param messageID path string true "Provider message ID"
// @Param format query string false "Response format" Enums(json,raw)
// @Success 200 {object} Email
// @Failure 404 {object} map[string]string
// @Router /api/accounts/{id}/emails/{messageID} [get]
func (a *App) getEmail(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	var email Email
	if a.db.Where("account_id = ? AND remote_id = ?", account.ID, c.Param("messageID")).First(&email).Error != nil {
		notFound(c)
		return
	}
	if c.DefaultQuery("format", "json") == "raw" {
		if email.Raw == "" {
			if !usesOAuth(account.Provider) {
				c.JSON(http.StatusConflict, gin.H{"error": "raw content is not cached; sync this mailbox again"})
				return
			}
			token, err := a.token(c.Request.Context(), &account)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth token could not be refreshed; reconnect the account"})
				return
			}
			email.Raw, err = rawMessage(c.Request.Context(), account, token, email.RemoteID)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			if err := a.db.Model(&email).Update("raw", email.Raw).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not cache raw email"})
				return
			}
		}
		raw, err := decodeRawEmail(email.Raw)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored raw email"})
			return
		}
		c.Data(http.StatusOK, "message/rfc822", raw)
		return
	}
	if email.Body == "" && account.Status == "connected" {
		var err error
		if (account.Provider == "imap" || account.Provider == "pop3") && email.Raw != "" {
			email.Body, err = bodyFromRawEmail(email.Raw)
		} else if usesOAuth(account.Provider) {
			var token *oauth2.Token
			token, err = a.token(c.Request.Context(), &account)
			if err == nil {
				email.Body, err = messageBody(c.Request.Context(), account, token, email.RemoteID)
			}
		}
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not load email body: " + safeError(err)})
			return
		}
		if err == nil && email.Body != "" {
			if err := a.db.Model(&email).Update("body", email.Body).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not cache email body"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, email)
}

func decodeRawEmail(value string) ([]byte, error) {
	raw, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(value)
	}
	return raw, err
}

func bodyFromRawEmail(raw string) (string, error) {
	data, err := decodeRawEmail(raw)
	if err != nil {
		return "", err
	}
	message, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	body, err := readLimited(message.Body, maxMessageBytes())
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (a *App) account(c *gin.Context) (Account, bool) {
	var account Account
	if a.db.Preload("Proxy").First(&account, c.Param("id")).Error != nil {
		notFound(c)
		return Account{}, false
	}
	if err := a.openAccount(&account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "credential decryption failed"})
		return Account{}, false
	}
	return account, true
}
func (a *App) token(ctx context.Context, account *Account) (*oauth2.Token, error) {
	httpClient, err := providerHTTPClient(*account)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	t := &oauth2.Token{AccessToken: account.AccessToken, RefreshToken: account.RefreshToken, Expiry: account.TokenExpiry}
	source := oauthConfig(account.Provider, account.OAuthClientID).TokenSource(ctx, t)
	next, err := tokenWithRetry(ctx, source)
	if err == nil && (next.AccessToken != account.AccessToken || !next.Expiry.Equal(account.TokenExpiry)) {
		account.AccessToken, account.RefreshToken, account.TokenExpiry = next.AccessToken, next.RefreshToken, next.Expiry
		if err := a.saveAccount(account); err != nil {
			return nil, err
		}
	}
	return next, err
}
func oauthConfig(provider, clientID string) *oauth2.Config {
	redirect := env("OAUTH_REDIRECT_URL", "http://localhost:8080/oauth/callback")
	if provider == "gmail" {
		return &oauth2.Config{ClientID: clientID, ClientSecret: env("GMAIL_CLIENT_SECRET", ""), RedirectURL: redirect, Endpoint: google.Endpoint, Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"}}
	}
	return &oauth2.Config{ClientID: clientID, ClientSecret: env("MICROSOFT_CLIENT_SECRET", ""), RedirectURL: redirect, Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token"}, Scopes: []string{"offline_access", "https://graph.microsoft.com/Mail.Read"}}
}

func gmailMessages(ctx context.Context, account Account, token *oauth2.Token, folder string) ([]Email, error) {
	client, err := clientFor(account, token)
	if err != nil {
		return nil, err
	}
	params := url.Values{"maxResults": {"100"}, "includeSpamTrash": {"true"}}
	if folder != "all" {
		params.Add("labelIds", folder)
	}
	emails := []Email{}
	for page := 0; page < positive(env("SYNC_MAX_PAGES", "5"), 5); page++ {
		var list struct {
			Messages      []struct{ ID, ThreadID string } `json:"messages"`
			NextPageToken string                          `json:"nextPageToken"`
		}
		if err := getJSON(ctx, client, "https://gmail.googleapis.com/gmail/v1/users/me/messages?"+params.Encode(), &list); err != nil {
			return nil, err
		}
		for _, item := range list.Messages {
			var m struct {
				ID, ThreadID, Snippet string
				LabelIDs              []string `json:"labelIds"`
				InternalDate          string   `json:"internalDate"`
				Payload               struct {
					Headers []struct{ Name, Value string }
				}
			}
			if err := getJSON(ctx, client, "https://gmail.googleapis.com/gmail/v1/users/me/messages/"+url.PathEscape(item.ID)+"?format=full", &m); err != nil {
				return nil, err
			}
			received, _ := strconv.ParseInt(m.InternalDate, 10, 64)
			e := Email{AccountID: account.ID, RemoteID: m.ID, Folder: folder, ThreadID: m.ThreadID, Preview: m.Snippet, ReceivedAt: time.UnixMilli(received), IsRead: true}
			if folder == "all" {
				e.Folder = gmailFolder(m.LabelIDs)
			}
			for _, h := range m.Payload.Headers {
				switch strings.ToLower(h.Name) {
				case "from":
					e.From = h.Value
				case "to":
					e.To = h.Value
				case "subject":
					e.Subject = h.Value
				}
			}
			for _, label := range m.LabelIDs {
				if label == "UNREAD" {
					e.IsRead = false
				}
			}
			emails = append(emails, e)
		}
		if list.NextPageToken == "" {
			break
		}
		params.Set("pageToken", list.NextPageToken)
	}
	return emails, nil
}
func microsoftMessages(ctx context.Context, account Account, token *oauth2.Token, folder string) ([]Email, error) {
	client, err := clientFor(account, token)
	if err != nil {
		return nil, err
	}
	endpoint := "https://graph.microsoft.com/v1.0/me/messages"
	if folder != "all" {
		endpoint = "https://graph.microsoft.com/v1.0/me/mailFolders/" + url.PathEscape(folder) + "/messages"
	}
	params := url.Values{"$top": {"100"}, "$orderby": {"receivedDateTime desc"}, "$select": {"id,conversationId,subject,bodyPreview,isRead,receivedDateTime,sentDateTime,parentFolderId,from,toRecipients"}}
	emails := []Email{}
	for page := 0; page < positive(env("SYNC_MAX_PAGES", "5"), 5); page++ {
		var list struct {
			Value []struct {
				ID, ConversationID, Subject, BodyPreview, ParentFolderID string
				IsRead                                                   bool                                              `json:"isRead"`
				ReceivedDateTime                                         time.Time                                         `json:"receivedDateTime"`
				SentDateTime                                             time.Time                                         `json:"sentDateTime"`
				From                                                     struct{ EmailAddress struct{ Address string } }   `json:"from"`
				ToRecipients                                             []struct{ EmailAddress struct{ Address string } } `json:"toRecipients"`
			} `json:"value"`
			NextLink string `json:"@odata.nextLink"`
		}
		requestURL := endpoint + "?" + params.Encode()
		if page > 0 {
			requestURL = endpoint
		}
		if err := getJSON(ctx, client, requestURL, &list); err != nil {
			return nil, err
		}
		for _, m := range list.Value {
			to := make([]string, 0, len(m.ToRecipients))
			for _, r := range m.ToRecipients {
				to = append(to, r.EmailAddress.Address)
			}
			messageFolder := m.ParentFolderID
			if folder != "all" {
				messageFolder = folder
			}
			receivedAt := m.ReceivedDateTime
			if receivedAt.IsZero() {
				receivedAt = m.SentDateTime
			}
			emails = append(emails, Email{AccountID: account.ID, RemoteID: m.ID, Folder: messageFolder, ThreadID: m.ConversationID, Subject: m.Subject, Preview: m.BodyPreview, From: m.From.EmailAddress.Address, To: strings.Join(to, ", "), ReceivedAt: receivedAt, IsRead: m.IsRead})
		}
		if list.NextLink == "" {
			break
		}
		endpoint = list.NextLink
		params = url.Values{}
	}
	return emails, nil
}

func gmailFolder(labels []string) string {
	for _, label := range labels {
		if label == "SPAM" || label == "INBOX" || label == "SENT" || label == "DRAFT" || label == "TRASH" {
			return label
		}
	}
	return "ALL"
}
func gmailFolders(ctx context.Context, account Account, token *oauth2.Token) ([]Folder, error) {
	var result struct{ Labels []struct{ ID, Name string } }
	client, err := clientFor(account, token)
	if err != nil {
		return nil, err
	}
	err = getJSON(ctx, client, "https://gmail.googleapis.com/gmail/v1/users/me/labels", &result)
	if err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(result.Labels))
	for _, label := range result.Labels {
		folders = append(folders, Folder{ID: label.ID, Name: label.Name})
	}
	return folders, nil
}
func microsoftFolders(ctx context.Context, account Account, token *oauth2.Token) ([]Folder, error) {
	var result struct {
		Value []struct{ ID, DisplayName string } `json:"value"`
	}
	client, err := clientFor(account, token)
	if err != nil {
		return nil, err
	}
	err = getJSON(ctx, client, "https://graph.microsoft.com/v1.0/me/mailFolders?$top=999&includeHiddenFolders=true", &result)
	if err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(result.Value))
	for _, folder := range result.Value {
		folders = append(folders, Folder{ID: folder.ID, Name: folder.DisplayName})
	}
	return folders, nil
}
func rawMessage(ctx context.Context, account Account, token *oauth2.Token, id string) (string, error) {
	client, err := clientFor(account, token)
	if err != nil {
		return "", err
	}
	if account.Provider == "gmail" {
		var message struct {
			Raw string `json:"raw"`
		}
		err := getJSON(ctx, client, "https://gmail.googleapis.com/gmail/v1/users/me/messages/"+url.PathEscape(id)+"?format=raw", &message)
		return message.Raw, err
	}
	response, err := providerGet(ctx, client, "https://graph.microsoft.com/v1.0/me/messages/"+url.PathEscape(id)+"/$value")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := readLimited(response.Body, 4096)
		return "", fmt.Errorf("provider returned %s: %s", response.Status, body)
	}
	body, err := readLimited(response.Body, maxMessageBytes())
	return base64.RawURLEncoding.EncodeToString(body), err
}

type gmailPart struct {
	MimeType string `json:"mimeType"`
	Body     struct {
		Data string `json:"data"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}

func messageBody(ctx context.Context, account Account, token *oauth2.Token, id string) (string, error) {
	client, err := clientFor(account, token)
	if err != nil {
		return "", err
	}
	if account.Provider == "gmail" {
		var message struct {
			Payload gmailPart `json:"payload"`
		}
		if err := getJSON(ctx, client, "https://gmail.googleapis.com/gmail/v1/users/me/messages/"+url.PathEscape(id)+"?format=full", &message); err != nil {
			return "", err
		}
		return gmailText(message.Payload), nil
	}
	var message struct {
		Body struct {
			Content string `json:"content"`
		} `json:"body"`
	}
	err = getJSON(ctx, client, "https://graph.microsoft.com/v1.0/me/messages/"+url.PathEscape(id)+"?$select=body", &message)
	return message.Body.Content, err
}

func gmailText(part gmailPart) string {
	for _, child := range part.Parts {
		if text := gmailText(child); text != "" {
			return text
		}
	}
	if part.MimeType != "text/plain" && part.MimeType != "text/html" || part.Body.Data == "" {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(part.Body.Data)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(part.Body.Data)
	}
	if err != nil {
		return ""
	}
	return string(decoded)
}
func clientFor(account Account, token *oauth2.Token) (*http.Client, error) {
	base, err := providerHTTPClient(account)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: providerTimeout,
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base:   base.Transport,
		},
	}, nil
}

type providerStatusError struct {
	Status int
}

func (err *providerStatusError) Error() string {
	return fmt.Sprintf("provider returned HTTP %d", err.Status)
}

func tokenWithRetry(ctx context.Context, source oauth2.TokenSource) (*oauth2.Token, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		token, err := source.Token()
		if err == nil {
			return token, nil
		}
		lastErr = err
		if attempt < 3 {
			if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func providerGet(ctx context.Context, client *http.Client, endpoint string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(req)
		if err == nil && !retryableStatus(response.StatusCode) {
			return response, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = &providerStatusError{Status: response.StatusCode}
			response.Body.Close()
		}
		if attempt < 3 {
			if err := waitForRetry(ctx, retryDelay(attempt, responseHeader(response, "Retry-After"))); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func responseHeader(response *http.Response, key string) string {
	if response == nil {
		return ""
	}
	return response.Header.Get(key)
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	delay := time.Second << attempt
	return delay + time.Duration(time.Now().UnixNano()%400)*time.Millisecond
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("provider response exceeds the %d byte limit", limit)
	}
	return body, nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	response, err := providerGet(ctx, client, endpoint)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := readLimited(response.Body, 4096)
		return fmt.Errorf("provider returned %s: %s", response.Status, body)
	}
	body, err := readLimited(response.Body, maxMessageBytes())
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
