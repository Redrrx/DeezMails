package app

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"deezmails/internal/models"
	"deezmails/internal/validation"

	"github.com/gin-gonic/gin"
)

const maxDisplayedEmailBodyBytes = 1 << 20

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
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	page = min(page, 1_000_000)
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if err != nil || size < 1 {
		size = 50
	}
	size = min(size, 100)
	sort := c.DefaultQuery("sort", "receivedAt")
	direction := strings.ToLower(c.DefaultQuery("direction", "desc"))
	allowed := map[string]string{"receivedAt": "received_at", "subject": "subject", "from": "\"from\"", "createdAt": "created_at"}
	column, exists := allowed[sort]
	if !exists || (direction != "asc" && direction != "desc") {
		badRequest(c, "invalid sort or direction")
		return
	}
	var total int64
	q := a.requestDB(c).Model(&models.Email{}).Where("account_id = ?", account.ID)
	if folder := c.DefaultQuery("folder", "all"); folder != "all" {
		if len(folder) > 1024 || validation.HasLineBreakOrNUL(folder) {
			badRequest(c, "invalid folder")
			return
		}
		q = q.Where("folder = ?", folder)
	}
	if text := c.Query("q"); text != "" {
		if len(text) > 200 || validation.HasLineBreakOrNUL(text) {
			badRequest(c, "search query is too long or invalid")
			return
		}
		q = q.Where("(subject LIKE ? OR \"from\" LIKE ? OR preview LIKE ?)", "%"+text+"%", "%"+text+"%", "%"+text+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		internalError(c, "could not count emails", err)
		return
	}
	var emails []models.Email
	if err := q.Select("id", "account_id", "remote_id", "folder", "thread_id", "from", "to", "subject", "preview", "received_at", "is_read", "created_at").Order(column + " " + direction).Limit(size).Offset((page - 1) * size).Find(&emails).Error; err != nil {
		internalError(c, "could not load emails", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": emails, "page": page, "pageSize": size, "total": total})
}

// listFolders godoc
// @Summary List available mailbox folders
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Success 200 {array} models.Folder
// @Failure 401,403,404,409,429,502,503 {object} models.ErrorResponse
// @Router /api/accounts/{id}/folders [get]
func (a *App) listFolders(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	if !account.Enabled || account.Status != models.AccountStatusConnected {
		folders, err := a.cachedFolders(c.Request.Context(), account)
		if err != nil {
			internalError(c, "could not load cached folders", err)
			return
		}
		if len(folders) == 0 && (account.Provider == models.ProviderIMAP || account.Provider == models.ProviderPOP3) {
			folders = []models.Folder{{ID: "INBOX", Name: "Inbox"}}
		}
		c.JSON(http.StatusOK, folders)
		return
	}
	if err := a.validateMailTransport(account); err != nil {
		a.respondAccountMailError(c, &account, err)
		return
	}
	if account.Provider == models.ProviderPOP3 {
		c.JSON(http.StatusOK, []models.Folder{{ID: "INBOX", Name: "Inbox"}})
		return
	}
	if account.Provider == models.ProviderIMAP {
		folders, err := a.imapFolders(c.Request.Context(), account)
		if err != nil {
			a.respondAccountMailError(c, &account, err)
			return
		}
		c.JSON(http.StatusOK, folders)
		return
	}
	token, err := a.token(c.Request.Context(), &account)
	if err != nil {
		a.respondAccountMailError(c, &account, err)
		return
	}
	if account.Provider == models.ProviderGmail {
		folders, err := a.gmailFolders(c.Request.Context(), account, token)
		if err != nil {
			a.respondAccountMailError(c, &account, err)
			return
		}
		c.JSON(http.StatusOK, folders)
		return
	}
	folders, err := a.microsoftFolders(c.Request.Context(), account, token)
	if err != nil {
		a.respondAccountMailError(c, &account, err)
		return
	}
	c.JSON(http.StatusOK, folders)
}

func (a *App) cachedFolders(ctx context.Context, account models.Account) ([]models.Folder, error) {
	var names []string
	if err := a.db.WithContext(ctx).Model(&models.Email{}).Where("account_id = ?", account.ID).Distinct("folder").Order("folder").Limit(a.config.SyncMaxFolders).Pluck("folder", &names).Error; err != nil {
		return nil, err
	}
	folders := make([]models.Folder, 0, len(names))
	for _, name := range names {
		if name != "" {
			folders = append(folders, models.Folder{ID: name, Name: name})
		}
	}
	return folders, nil
}

// getEmail godoc
// @Summary Get an email in JSON or raw RFC 822 format
// @Tags emails
// @Produce json
// @Param id path int true "Account ID"
// @Param messageID path string true "Provider message ID"
// @Param format query string false "Response format" Enums(json,raw)
// @Success 200 {object} models.Email
// @Failure 404 {object} map[string]string
// @Failure 401,403,409,429,502,503 {object} models.ErrorResponse
// @Router /api/accounts/{id}/emails/{messageID} [get]
func (a *App) getEmail(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "raw" {
		badRequest(c, "format must be json or raw")
		return
	}
	var email models.Email
	if err := a.requestDB(c).Where("account_id = ? AND remote_id = ?", account.ID, c.Param("messageID")).First(&email).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	needsBody := format == "json" && email.Body == "" && !email.BodyFetched && (email.Raw != "" || account.Status == models.AccountStatusConnected)
	needsRaw := format == "raw" || needsBody
	var raw []byte
	if needsRaw && email.Raw != "" {
		var err error
		raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(email.Raw, "="))
		if err != nil {
			internalError(c, "invalid stored raw email", err)
			return
		}
		if len(raw) == 0 {
			internalError(c, "invalid stored raw email", errors.New("decoded message is empty"))
			return
		}
	}
	if needsRaw && len(raw) == 0 && email.Raw == "" && usesOAuth(account.Provider) {
		token, err := a.token(c.Request.Context(), &account)
		if err != nil {
			a.respondAccountMailError(c, &account, err)
			return
		}
		raw, err = a.fetchRawMessage(c.Request.Context(), account, token, email.RemoteID)
		if err != nil {
			a.respondAccountMailError(c, &account, err)
			return
		}
		email.Raw = base64.RawURLEncoding.EncodeToString(raw)
		if err := a.requestDB(c).Model(&email).Update("raw", email.Raw).Error; err != nil {
			internalError(c, "could not cache raw email", err)
			return
		}
	}
	if format == "raw" {
		if len(raw) == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "raw content is not cached; sync this mailbox again"})
			return
		}
		c.Data(http.StatusOK, "message/rfc822", raw)
		return
	}
	if needsBody && len(raw) != 0 {
		var err error
		email.Body, err = extractMessageText(raw, a.config.MaxMessageBytes)
		if err != nil {
			respondMailError(c, &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionNone, Message: "Email content could not be parsed.", Cause: err})
			return
		}
		email.BodyFetched = true
		if err := a.requestDB(c).Model(&email).Updates(map[string]any{"body": email.Body, "body_fetched": true}).Error; err != nil {
			internalError(c, "could not cache email body", err)
			return
		}
	}
	if len(email.Body) > maxDisplayedEmailBodyBytes {
		email.Body = strings.ToValidUTF8(email.Body[:maxDisplayedEmailBodyBytes], "�")
		email.BodyTruncated = true
	}
	c.JSON(http.StatusOK, email)
}
