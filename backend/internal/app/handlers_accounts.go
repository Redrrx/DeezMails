package app

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"deezmails/internal/models"
	"deezmails/internal/validation"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// createAccount godoc
// @Summary Create mailbox account metadata
// @Tags accounts
// @Accept json
// @Produce json
// @Param account body models.AccountInput true "Account"
// @Success 201 {object} models.Account
// @Router /api/accounts [post]
func (a *App) createAccount(c *gin.Context) {
	var input models.AccountInput
	if bindJSON(c, &input) != nil {
		badRequest(c, "invalid account configuration")
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.RefreshToken = strings.TrimSpace(input.RefreshToken)
	input.IncomingHost = validation.NormalizeNetworkHost(strings.TrimSpace(input.IncomingHost))
	input.TLSMode = strings.ToLower(strings.TrimSpace(input.TLSMode))
	input.Folder = strings.TrimSpace(input.Folder)
	if input.Folder == "" {
		input.Folder = "INBOX"
	}
	if input.TLSMode == "" {
		input.TLSMode = models.TLSModeImplicit
	}
	if !validMailboxAddress(input.Email) || !validProvider(input.Provider) || len(input.ClientID) > 4096 || validation.HasLineBreakOrNUL(input.ClientID) || len(input.RefreshToken) > 128<<10 || validation.HasLineBreakOrNUL(input.RefreshToken) || len(input.Folder) > 1024 || validation.HasLineBreakOrNUL(input.Folder) || (usesOAuth(input.Provider) && input.ClientID == "") || !a.validIncoming(input) {
		badRequest(c, "invalid account configuration")
		return
	}
	account := models.Account{Email: input.Email, Provider: input.Provider, OAuthClientID: input.ClientID, MailboxPassword: input.Password, RefreshToken: input.RefreshToken, IncomingHost: input.IncomingHost, IncomingPort: input.IncomingPort, TLSMode: input.TLSMode, Folder: input.Folder, ProxyID: input.ProxyID}
	account.Enabled, account.SyncEnabled = true, true
	account.Status = models.AccountStatusDisconnected
	if account.RefreshToken != "" {
		account.Status = models.AccountStatusConnected
	}
	if account.ProxyID != nil {
		if err := a.requestDB(c).First(&models.Proxy{}, *account.ProxyID).Error; err != nil {
			handleLookupError(c, err)
			return
		}
	}
	if err := a.createAccountRecord(c.Request.Context(), &account); err != nil {
		databaseWriteError(c, "could not create account", err)
		return
	}
	if !usesOAuth(account.Provider) || account.RefreshToken != "" {
		if _, err := a.enqueueJob(c.Request.Context(), models.JobTypeVerifyConnection, account.ID, ""); err != nil {
			slog.Error("account created but connection check could not be queued", "accountID", account.ID, "error", err)
			account.LastError, account.LastErrorCode, account.LastErrorAction = "Connection check could not be queued. Try again.", models.MailErrorInternal, models.MailErrorActionRetry
			_ = a.saveAccount(c.Request.Context(), &account)
		}
	}
	c.JSON(http.StatusCreated, account)
}

// listAccounts godoc
// @Summary List mailbox accounts
// @Tags accounts
// @Produce json
// @Param provider query string false "Filter provider" Enums(gmail,microsoft)
// @Success 200 {array} models.Account
// @Router /api/accounts [get]
func (a *App) listAccounts(c *gin.Context) {
	var items []models.Account
	q := a.requestDB(c).Preload("Proxy").Order("id desc")
	if provider := c.Query("provider"); provider != "" {
		q = q.Where("provider = ?", provider)
	}
	if err := q.Find(&items).Error; err != nil {
		internalError(c, "could not load accounts", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

// getAccount godoc
// @Summary Get account
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {object} models.Account
// @Router /api/accounts/{id} [get]
func (a *App) getAccount(c *gin.Context) {
	var account models.Account
	if err := a.requestDB(c).Preload("Proxy").First(&account, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
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
// @Param account body models.AccountUpdateInput true "Account update"
// @Success 200 {object} models.Account
// @Router /api/accounts/{id} [put]
func (a *App) updateAccount(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	var in models.AccountUpdateInput
	if bindJSON(c, &in) != nil {
		badRequest(c, "invalid json")
		return
	}

	providerChanged := false
	if in.Provider != nil {
		provider := strings.ToLower(strings.TrimSpace(*in.Provider))
		if !validProvider(provider) {
			badRequest(c, "invalid provider")
			return
		}
		providerChanged = provider != account.Provider
		account.Provider = provider
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if !validMailboxAddress(email) {
			badRequest(c, "a valid email address is required")
			return
		}
		account.Email = email
	}
	if in.Enabled != nil {
		account.Enabled = *in.Enabled
	}
	if in.SyncEnabled != nil {
		account.SyncEnabled = *in.SyncEnabled
	}
	if in.ProxyID != nil {
		if *in.ProxyID == 0 {
			account.ProxyID, account.Proxy = nil, nil
		} else {
			var proxy models.Proxy
			if err := a.requestDB(c).First(&proxy, *in.ProxyID).Error; err != nil {
				handleLookupError(c, err)
				return
			}
			proxyID := proxy.ID
			account.ProxyID, account.Proxy = &proxyID, &proxy
		}
	}
	if in.Folder != nil {
		account.Folder = strings.TrimSpace(*in.Folder)
		if account.Folder == "" || len(account.Folder) > 1024 || validation.HasLineBreakOrNUL(account.Folder) {
			badRequest(c, "invalid folder")
			return
		}
	}

	connectionChanged := providerChanged || in.Email != nil || in.ClientID != nil || in.RefreshToken != nil || in.Password != nil || in.IncomingHost != nil || in.IncomingPort != nil || in.TLSMode != nil || in.Folder != nil || in.ProxyID != nil
	if usesOAuth(account.Provider) {
		if in.ClientID != nil {
			account.OAuthClientID = strings.TrimSpace(*in.ClientID)
		}
		if account.OAuthClientID == "" || len(account.OAuthClientID) > 4096 || validation.HasLineBreakOrNUL(account.OAuthClientID) {
			badRequest(c, "OAuth client ID is required")
			return
		}
		if in.RefreshToken != nil && strings.TrimSpace(*in.RefreshToken) != "" {
			refreshToken := strings.TrimSpace(*in.RefreshToken)
			if len(refreshToken) > 128<<10 || validation.HasLineBreakOrNUL(refreshToken) {
				badRequest(c, "invalid OAuth refresh token")
				return
			}
			account.RefreshToken = refreshToken
		}
		if providerChanged || in.ClientID != nil || in.RefreshToken != nil {
			account.AccessToken, account.OAuthState, account.OAuthCodeVerifier, account.OAuthStateExpiresAt = "", "", "", nil
			account.TokenExpiry = time.Time{}
		}
		account.MailboxPassword = ""
		account.IncomingHost, account.IncomingPort = "", 0
		account.TLSMode = models.TLSModeImplicit
	} else {
		if providerChanged {
			account.RefreshToken = ""
		}
		account.OAuthClientID, account.AccessToken, account.OAuthState, account.OAuthCodeVerifier = "", "", "", ""
		account.TokenExpiry = time.Time{}
		if in.Password != nil && *in.Password != "" {
			if len(*in.Password) > 64<<10 || validation.HasLineBreakOrNUL(*in.Password) {
				badRequest(c, "invalid mailbox password")
				return
			}
			account.MailboxPassword = *in.Password
		}
		if in.IncomingHost != nil {
			account.IncomingHost = validation.NormalizeNetworkHost(strings.TrimSpace(*in.IncomingHost))
		}
		if in.IncomingPort != nil {
			account.IncomingPort = *in.IncomingPort
		}
		if in.TLSMode != nil {
			account.TLSMode = strings.ToLower(strings.TrimSpace(*in.TLSMode))
		}
		if !a.validIncoming(models.AccountInput{Email: account.Email, Provider: account.Provider, Password: account.MailboxPassword, IncomingHost: account.IncomingHost, IncomingPort: account.IncomingPort, TLSMode: account.TLSMode}) {
			badRequest(c, "a host, valid port, security mode, and password are required for IMAP/POP3")
			return
		}
	}

	needsVerification := !usesOAuth(account.Provider) || account.RefreshToken != ""
	if connectionChanged {
		account.Status, account.LastError, account.LastErrorCode, account.LastErrorAction = models.AccountStatusDisconnected, "", "", ""
	}
	if err := a.saveAccount(c.Request.Context(), &account); err != nil {
		databaseWriteError(c, "could not save account", err)
		return
	}
	if connectionChanged && needsVerification {
		if _, err := a.enqueueJob(c.Request.Context(), models.JobTypeVerifyConnection, account.ID, ""); err != nil {
			internalError(c, "account saved but connection check could not be queued", err)
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
	var account models.Account
	if err := a.requestDB(c).First(&account, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	var running int64
	if err := a.requestDB(c).Model(&models.JobRun{}).Where("account_id = ? AND status = ?", account.ID, models.JobStatusRunning).Count(&running).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check account jobs"})
		return
	}
	if running > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "account has a running job; wait for it to finish, then remove it"})
		return
	}
	if err := a.removeQueuedJobs(c.Request.Context(), account.ID); err != nil {
		internalError(c, "could not remove queued account jobs", err)
		return
	}
	if err := a.requestDB(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", account.ID).Delete(&models.JobRun{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", account.ID).Delete(&models.Email{}).Error; err != nil {
			return err
		}
		return tx.Delete(&account).Error
	}); err != nil {
		internalError(c, "could not delete account", err)
		return
	}
	c.Status(http.StatusNoContent)
}
