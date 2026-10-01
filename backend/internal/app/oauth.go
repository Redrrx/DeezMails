package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"deezmails/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// reconnect godoc
// @Summary Reconnect a mailbox
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {object} map[string]string
// @Success 202 {object} models.JobRun
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
	if account.Status == models.AccountStatusConnected {
		c.JSON(http.StatusConflict, gin.H{"error": "account is already connected; sync it instead"})
		return
	}
	if !usesOAuth(account.Provider) {
		job, err := a.enqueueJob(c.Request.Context(), models.JobTypeVerifyConnection, account.ID, "")
		if err != nil {
			internalError(c, "could not queue connection check", err)
			return
		}
		c.JSON(http.StatusAccepted, job)
		return
	}
	if account.OAuthState != "" && account.OAuthStateExpiresAt != nil && account.OAuthStateExpiresAt.After(time.Now()) {
		c.JSON(http.StatusConflict, gin.H{"error": "OAuth approval is already in progress"})
		return
	}
	state := oauth2.GenerateVerifier()
	expires := time.Now().Add(10 * time.Minute)
	account.OAuthState = state
	account.OAuthCodeVerifier = oauth2.GenerateVerifier()
	account.OAuthStateExpiresAt = &expires
	if err := a.saveAccount(c.Request.Context(), &account); err != nil {
		internalError(c, "could not prepare OAuth connection", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": a.oauthConfig(account.Provider, account.OAuthClientID).AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(account.OAuthCodeVerifier))})
}

// oauthCallback godoc
// @Summary OAuth provider callback
// @Tags OAuth
// @Param state query string true "OAuth state"
// @Param code query string false "Authorization code"
// @Param error query string false "OAuth error code"
// @Success 200 {string} string "Account connected"
// @Router /oauth/callback [get]
func (a *App) oauthCallback(c *gin.Context) {
	state, code, authorizationError := c.Query("state"), c.Query("code"), c.Query("error")
	if state == "" || (code == "" && authorizationError == "") {
		c.String(http.StatusBadRequest, "invalid or expired OAuth callback")
		return
	}
	var account models.Account
	err := a.requestDB(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Proxy").Where("o_auth_state = ? AND o_auth_state_expires_at > ? AND o_auth_code_verifier <> ?", state, time.Now(), "").First(&account).Error; err != nil {
			return err
		}
		result := tx.Model(&models.Account{}).Where("id = ? AND o_auth_state = ?", account.ID, state).Updates(map[string]any{"o_auth_state": "", "o_auth_code_verifier": "", "o_auth_state_expires_at": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.String(http.StatusBadRequest, "invalid or expired OAuth callback")
		return
	}
	if err != nil {
		internalError(c, "could not validate OAuth callback", err)
		return
	}
	account.OAuthState, account.OAuthStateExpiresAt = "", nil
	if err := a.decryptAccountCredentials(&account); err != nil {
		internalError(c, "credential decryption failed", err)
		return
	}
	codeVerifier := account.OAuthCodeVerifier
	account.OAuthCodeVerifier = ""
	if authorizationError != "" {
		failure := &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Authorization was cancelled or denied. Start reconnect again.", Cause: errors.New(authorizationError)}
		switch authorizationError {
		case "temporarily_unavailable", "server_error":
			failure.Code, failure.Message = models.MailErrorUnavailable, "OAuth provider is temporarily unavailable. Start reconnect again."
		case "invalid_request", "unauthorized_client", "invalid_scope":
			failure.Code, failure.Action, failure.Message = models.MailErrorConfiguration, models.MailErrorActionContactAdmin, "OAuth application configuration was rejected. Contact the administrator."
		case "access_denied":
		default:
			failure.Message = "Authorization failed. Start reconnect again."
		}
		setAccountFailure(&account, failure)
		if err := a.saveAccount(c.Request.Context(), &account); err != nil {
			internalError(c, "could not save OAuth failure", err)
			return
		}
		slog.Warn("OAuth authorization failed", "accountID", account.ID, "providerError", authorizationError, "description", c.Query("error_description"))
		c.String(http.StatusBadRequest, failure.Message)
		return
	}
	providerClient, err := providerHTTPClient(account)
	if err != nil {
		internalError(c, "could not configure OAuth connection", err)
		return
	}
	exchangeContext := context.WithValue(c.Request.Context(), oauth2.HTTPClient, providerClient)
	token, err := a.oauthConfig(account.Provider, account.OAuthClientID).Exchange(exchangeContext, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		failure := setAccountFailure(&account, err)
		if saveErr := a.saveAccount(c.Request.Context(), &account); saveErr != nil {
			internalError(c, "could not save OAuth failure", saveErr)
			return
		}
		slog.Warn("OAuth token exchange failed", "accountID", account.ID, "error", err)
		status := http.StatusBadRequest
		if failure.Action == models.MailErrorActionRetry {
			status = http.StatusServiceUnavailable
		}
		c.String(status, failure.Message)
		return
	}
	refreshToken := token.RefreshToken
	if refreshToken == "" {
		refreshToken = account.RefreshToken
	}
	if refreshToken == "" {
		failure := &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "OAuth provider did not return durable access. Start reconnect again.", Cause: errors.New("OAuth provider did not return a refresh token")}
		setAccountFailure(&account, failure)
		if saveErr := a.saveAccount(c.Request.Context(), &account); saveErr != nil {
			internalError(c, "could not save OAuth failure", saveErr)
			return
		}
		c.String(http.StatusBadRequest, failure.Message)
		return
	}
	account.AccessToken, account.RefreshToken, account.TokenExpiry, account.Status, account.LastError = token.AccessToken, refreshToken, token.Expiry, models.AccountStatusConnected, ""
	account.LastErrorCode, account.LastErrorAction = "", ""
	if err := a.saveAccount(c.Request.Context(), &account); err != nil {
		internalError(c, "could not save OAuth token", err)
		return
	}
	if account.Enabled && account.SyncEnabled {
		if _, err := a.enqueueJob(c.Request.Context(), models.JobTypeSyncAccount, account.ID, folderAll); err != nil {
			slog.Error("connected account but could not queue initial sync", "accountID", account.ID, "error", err)
			c.String(http.StatusOK, "Connected %s, but the initial sync could not be queued. You may close this page.", account.Email)
			return
		}
	}
	c.String(http.StatusOK, "Connected %s. You may close this page.", account.Email)
}

func (a *App) account(c *gin.Context) (models.Account, bool) {
	var account models.Account
	if err := a.requestDB(c).Preload("Proxy").First(&account, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return models.Account{}, false
	}
	if err := a.decryptAccountCredentials(&account); err != nil {
		internalError(c, "credential decryption failed", err)
		return models.Account{}, false
	}
	return account, true
}

func (a *App) token(ctx context.Context, account *models.Account) (*oauth2.Token, error) {
	httpClient, err := providerHTTPClient(*account)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	t := &oauth2.Token{AccessToken: account.AccessToken, RefreshToken: account.RefreshToken, Expiry: account.TokenExpiry}
	source := a.oauthConfig(account.Provider, account.OAuthClientID).TokenSource(ctx, t)
	next, err := source.Token()
	if err != nil {
		return nil, normalizeMailError(err)
	}
	if next.AccessToken != account.AccessToken || !next.Expiry.Equal(account.TokenExpiry) {
		refreshToken := next.RefreshToken
		if refreshToken == "" {
			refreshToken = account.RefreshToken
		}
		account.AccessToken, account.RefreshToken, account.TokenExpiry = next.AccessToken, refreshToken, next.Expiry
		if err := a.saveAccountTokens(ctx, account); err != nil {
			return nil, err
		}
	}
	return next, nil
}

func (a *App) oauthConfig(provider, clientID string) *oauth2.Config {
	redirect := a.config.OAuthRedirectURL
	if provider == models.ProviderGmail {
		return &oauth2.Config{ClientID: clientID, ClientSecret: a.config.GmailClientSecret, RedirectURL: redirect, Endpoint: google.Endpoint, Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"}}
	}
	return &oauth2.Config{ClientID: clientID, ClientSecret: a.config.MicrosoftClientSecret, RedirectURL: redirect, Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token"}, Scopes: []string{"offline_access", "https://graph.microsoft.com/Mail.Read"}}
}
