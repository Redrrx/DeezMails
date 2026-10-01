package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deezmails/internal/models"

	"github.com/emersion/go-imap/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

type mailFailure struct {
	Code       models.MailErrorCode
	Action     models.MailErrorAction
	Message    string
	RetryAfter time.Duration
	Cause      error
}

func (failure *mailFailure) Error() string { return failure.Message }
func (failure *mailFailure) Unwrap() error { return failure.Cause }

type microsoftHTTPError struct {
	Status     int
	Codes      []string
	Message    string
	RetryAfter string
}

const microsoftBandwidthLimitExceeded = 509

func (err *microsoftHTTPError) Error() string {
	return fmt.Sprintf("Microsoft Graph returned HTTP %d (%s): %s", err.Status, strings.Join(err.Codes, ", "), err.Message)
}

func normalizeMailError(err error) *mailFailure {
	if failure, ok := errors.AsType[*mailFailure](err); ok {
		return failure
	}
	if errors.Is(err, context.Canceled) {
		return &mailFailure{Code: models.MailErrorCancelled, Action: models.MailErrorActionNone, Message: "Operation was cancelled.", Cause: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &mailFailure{Code: models.MailErrorNetwork, Action: models.MailErrorActionRetry, Message: "Mail provider timed out. Try again.", Cause: err}
	}
	if providerError, ok := errors.AsType[*googleapi.Error](err); ok {
		retryAfter := parseRetryAfter(providerError.Header.Get("Retry-After"))
		for _, item := range providerError.Errors {
			switch strings.ToLower(item.Reason) {
			case "autherror", "invalidcredentials":
				return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Google authorization expired. Reconnect the account.", Cause: err}
			case "userratelimitexceeded", "ratelimitexceeded":
				return &mailFailure{Code: models.MailErrorRateLimit, Action: models.MailErrorActionRetry, Message: "Gmail rate limit reached. Try again later.", RetryAfter: retryAfter, Cause: err}
			case "backenderror":
				return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "Gmail is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
			case "dailylimitexceeded":
				return &mailFailure{Code: models.MailErrorQuota, Action: models.MailErrorActionContactAdmin, Message: "Gmail API quota is exhausted. Contact the administrator.", Cause: err}
			case "quotaexceeded":
				if providerError.Code == http.StatusTooManyRequests {
					return &mailFailure{Code: models.MailErrorRateLimit, Action: models.MailErrorActionRetry, Message: "Gmail rate limit reached. Try again later.", RetryAfter: retryAfter, Cause: err}
				}
				return &mailFailure{Code: models.MailErrorQuota, Action: models.MailErrorActionContactAdmin, Message: "Gmail API quota is exhausted. Contact the administrator.", Cause: err}
			case "domainpolicy":
				return &mailFailure{Code: models.MailErrorPolicy, Action: models.MailErrorActionContactAdmin, Message: "Your Google Workspace policy blocks Gmail access. Contact the administrator.", Cause: err}
			case "insufficientpermissions", "forbidden":
				return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionReconnect, Message: "Gmail permission is missing. Reconnect the account.", Cause: err}
			}
		}
		switch {
		case providerError.Code == http.StatusUnauthorized:
			return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Google authorization expired. Reconnect the account.", Cause: err}
		case providerError.Code == http.StatusTooManyRequests:
			return &mailFailure{Code: models.MailErrorRateLimit, Action: models.MailErrorActionRetry, Message: "Gmail rate limit reached. Try again later.", RetryAfter: retryAfter, Cause: err}
		case providerError.Code == http.StatusForbidden:
			return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionContactAdmin, Message: "Gmail denied mailbox access. Contact the administrator.", Cause: err}
		case providerError.Code == http.StatusNotFound:
			return &mailFailure{Code: models.MailErrorNotFound, Action: models.MailErrorActionNone, Message: "Gmail message or mailbox was not found.", Cause: err}
		case providerError.Code >= 500:
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "Gmail is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
		default:
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionContactAdmin, Message: "Gmail rejected the request. Contact the administrator.", Cause: err}
		}
	}
	if providerError, ok := errors.AsType[*microsoftHTTPError](err); ok {
		retryAfter := parseRetryAfter(providerError.RetryAfter)
		for i := len(providerError.Codes) - 1; i >= 0; i-- {
			switch strings.ToLower(providerError.Codes[i]) {
			case "invalidauthenticationtoken", "authenticationerror", "invalidgrant":
				return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Microsoft authorization expired. Reconnect the account.", Cause: err}
			case "insufficient_claims":
				return &mailFailure{Code: models.MailErrorPolicy, Action: models.MailErrorActionContactAdmin, Message: "Microsoft conditional-access policy requires administrator action.", Cause: err}
			case "erroraccessdenied", "authorization_requestdenied", "accessdenied":
				return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionContactAdmin, Message: "Microsoft denied mailbox access. Contact the administrator.", Cause: err}
			case "mailboxnotenabledforrestapi", "errormailboxconfiguration", "errorinvaliduser":
				return &mailFailure{Code: models.MailErrorPolicy, Action: models.MailErrorActionContactAdmin, Message: "Microsoft 365 mailbox access is disabled or unlicensed. Contact the administrator.", Cause: err}
			case "errorquotaexceeded", "mailboxfull":
				return &mailFailure{Code: models.MailErrorQuota, Action: models.MailErrorActionContactAdmin, Message: "Microsoft mailbox quota is exhausted. Contact the administrator.", Cause: err}
			case "toomanyrequests", "errortoobusy", "errorserverbusy", "errortoomanyobjectsopened", "serviceunavailable", "servicenotavailable", "errortimeout", "directory_concurrencyviolation":
				return &mailFailure{Code: models.MailErrorRateLimit, Action: models.MailErrorActionRetry, Message: "Microsoft is temporarily busy. Try again later.", RetryAfter: retryAfter, Cause: err}
			case "erroritemnotfound", "errorfoldernotfound", "errormailboxnotfound", "resourcenotfound":
				return &mailFailure{Code: models.MailErrorNotFound, Action: models.MailErrorActionNone, Message: "Microsoft message or mailbox was not found.", Cause: err}
			case "errorinvalididmalformed", "errorinvalidrequest", "request_badrequest":
				return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionContactAdmin, Message: "Microsoft rejected the request. Contact the administrator.", Cause: err}
			}
		}
		switch {
		case providerError.Status == http.StatusUnauthorized:
			return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Microsoft authorization expired. Reconnect the account.", Cause: err}
		case providerError.Status == http.StatusTooManyRequests || providerError.Status == microsoftBandwidthLimitExceeded:
			return &mailFailure{Code: models.MailErrorRateLimit, Action: models.MailErrorActionRetry, Message: "Microsoft is temporarily busy. Try again later.", RetryAfter: retryAfter, Cause: err}
		case providerError.Status == http.StatusInsufficientStorage:
			return &mailFailure{Code: models.MailErrorQuota, Action: models.MailErrorActionContactAdmin, Message: "Microsoft mailbox storage quota is exhausted. Contact the administrator.", Cause: err}
		case providerError.Status == http.StatusPaymentRequired:
			return &mailFailure{Code: models.MailErrorPolicy, Action: models.MailErrorActionContactAdmin, Message: "Microsoft 365 licensing or payment action is required. Contact the administrator.", Cause: err}
		case providerError.Status == http.StatusForbidden:
			return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionContactAdmin, Message: "Microsoft denied mailbox access. Contact the administrator.", Cause: err}
		case providerError.Status == http.StatusNotFound || providerError.Status == http.StatusGone:
			return &mailFailure{Code: models.MailErrorNotFound, Action: models.MailErrorActionNone, Message: "Microsoft message or mailbox was not found.", Cause: err}
		case providerError.Status == http.StatusLocked || providerError.Status == http.StatusRequestTimeout:
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "Microsoft mailbox is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
		case providerError.Status == http.StatusNotImplemented:
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionContactAdmin, Message: "Microsoft does not support this mailbox operation. Contact the administrator.", Cause: err}
		case providerError.Status >= 500:
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "Microsoft is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
		default:
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionContactAdmin, Message: "Microsoft rejected the request. Contact the administrator.", Cause: err}
		}
	}
	if tokenError, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		status := 0
		retryAfter := time.Duration(0)
		if tokenError.Response != nil {
			status = tokenError.Response.StatusCode
			retryAfter = parseRetryAfter(tokenError.Response.Header.Get("Retry-After"))
		}
		switch strings.ToLower(tokenError.ErrorCode) {
		case "invalid_grant", "access_denied", "interaction_required", "consent_required", "login_required":
			return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Authorization expired or was revoked. Reconnect the account.", Cause: err}
		case "invalid_client", "unauthorized_client", "invalid_scope":
			return &mailFailure{Code: models.MailErrorConfiguration, Action: models.MailErrorActionContactAdmin, Message: "OAuth application configuration was rejected. Contact the administrator.", Cause: err}
		case "temporarily_unavailable", "server_error":
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "OAuth provider is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
		}
		if status == http.StatusTooManyRequests || status >= 500 {
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "OAuth provider is temporarily unavailable. Try again later.", RetryAfter: retryAfter, Cause: err}
		}
		return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionReconnect, Message: "Authorization failed. Reconnect the account.", Cause: err}
	}
	if protocolError, ok := errors.AsType[*imap.Error](err); ok {
		switch protocolError.Code {
		case imap.ResponseCodeAuthenticationFailed, imap.ResponseCodeAuthorizationFailed, imap.ResponseCodeExpired:
			return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionEditAccount, Message: "IMAP rejected the mailbox credentials. Edit the account credentials.", Cause: err}
		case imap.ResponseCodePrivacyRequired:
			return &mailFailure{Code: models.MailErrorTLS, Action: models.MailErrorActionEditAccount, Message: "IMAP requires a secure connection. Edit the account security mode.", Cause: err}
		case imap.ResponseCodeNoPerm:
			return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionContactAdmin, Message: "IMAP denied mailbox access. Contact the mail administrator.", Cause: err}
		case imap.ResponseCodeOverQuota, imap.ResponseCodeLimit:
			return &mailFailure{Code: models.MailErrorQuota, Action: models.MailErrorActionContactAdmin, Message: "IMAP mailbox quota or server limit was reached. Contact the mail administrator.", Cause: err}
		case imap.ResponseCodeUnavailable, imap.ResponseCodeServerBug, imap.ResponseCodeInUse, imap.ResponseCodeTooMany:
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "IMAP server is temporarily unavailable. Try again later.", Cause: err}
		case imap.ResponseCodeNonExistent:
			return &mailFailure{Code: models.MailErrorNotFound, Action: models.MailErrorActionEditAccount, Message: "IMAP mailbox does not exist. Edit the account folder.", Cause: err}
		case imap.ResponseCodeContactAdmin, imap.ResponseCodeCorruption:
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionContactAdmin, Message: "IMAP server reported a mailbox problem. Contact the mail administrator.", Cause: err}
		default:
			if protocolError.Type == imap.StatusResponseTypeBye {
				return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "IMAP server closed the connection. Try again later.", Cause: err}
			}
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionNone, Message: "IMAP server rejected the operation.", Cause: err}
		}
	}
	if protocolError, ok := errors.AsType[*pop3Error](err); ok {
		switch protocolError.Code {
		case "AUTH":
			return &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionEditAccount, Message: "POP3 rejected the mailbox credentials. Edit the account credentials.", Cause: err}
		case "SYS/TEMP", "IN-USE", "LOGIN-DELAY":
			return &mailFailure{Code: models.MailErrorUnavailable, Action: models.MailErrorActionRetry, Message: "POP3 server is temporarily unavailable. Try again later.", Cause: err}
		case "SYS/PERM":
			return &mailFailure{Code: models.MailErrorPermission, Action: models.MailErrorActionContactAdmin, Message: "POP3 server rejected the operation. Contact the mail administrator.", Cause: err}
		default:
			return &mailFailure{Code: models.MailErrorProtocol, Action: models.MailErrorActionNone, Message: "POP3 server rejected the operation.", Cause: err}
		}
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return &mailFailure{Code: models.MailErrorResponseTooLarge, Action: models.MailErrorActionContactAdmin, Message: "Mail provider response exceeded the configured size limit.", Cause: err}
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return &mailFailure{Code: models.MailErrorTLS, Action: models.MailErrorActionEditAccount, Message: "Mail server TLS certificate could not be verified. Check the account server name.", Cause: err}
	}
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return &mailFailure{Code: models.MailErrorTLS, Action: models.MailErrorActionEditAccount, Message: "Mail server TLS mode is incorrect. Edit the account security mode.", Cause: err}
	}
	if dnsError, ok := errors.AsType[*net.DNSError](err); ok {
		if dnsError.IsNotFound {
			return &mailFailure{Code: models.MailErrorConfiguration, Action: models.MailErrorActionEditAccount, Message: "Mail server hostname was not found. Edit the account server name.", Cause: err}
		}
		return &mailFailure{Code: models.MailErrorNetwork, Action: models.MailErrorActionRetry, Message: "Mail server lookup failed. Try again later.", Cause: err}
	}
	if _, ok := errors.AsType[net.Error](err); ok || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &mailFailure{Code: models.MailErrorNetwork, Action: models.MailErrorActionRetry, Message: "Mail server connection failed. Try again later.", Cause: err}
	}
	if databaseError, ok := errors.AsType[sqlite3.Error](err); ok && (databaseError.Code == sqlite3.ErrBusy || databaseError.Code == sqlite3.ErrLocked) {
		return &mailFailure{Code: models.MailErrorInternal, Action: models.MailErrorActionRetry, Message: "Database is temporarily busy. Try again later.", Cause: err}
	}
	if databaseError, ok := errors.AsType[*pgconn.PgError](err); ok {
		state := databaseError.SQLState()
		if strings.HasPrefix(state, "08") || state == "40001" || state == "40P01" || state == "53300" || state == "57P03" {
			return &mailFailure{Code: models.MailErrorInternal, Action: models.MailErrorActionRetry, Message: "Database is temporarily unavailable. Try again later.", Cause: err}
		}
	}
	return &mailFailure{Code: models.MailErrorInternal, Action: models.MailErrorActionNone, Message: "Mailbox operation failed.", Cause: err}
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds > 0 {
		if seconds >= int64((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		return min(max(time.Until(retryAt), 0), 24*time.Hour)
	}
	return 0
}

func setAccountFailure(account *models.Account, err error) *mailFailure {
	failure := normalizeMailError(err)
	account.LastError, account.LastErrorCode, account.LastErrorAction = failure.Message, failure.Code, failure.Action
	if failure.Action == models.MailErrorActionReconnect {
		account.Status = models.AccountStatusReconnectRequired
	} else if failure.Action != models.MailErrorActionNone {
		account.Status = models.AccountStatusError
	}
	return failure
}

func respondMailError(c *gin.Context, err error) {
	failure := normalizeMailError(err)
	status := http.StatusBadGateway
	switch failure.Code {
	case models.MailErrorAuthentication:
		status = http.StatusUnauthorized
	case models.MailErrorPermission, models.MailErrorPolicy:
		status = http.StatusForbidden
	case models.MailErrorNotFound:
		status = http.StatusNotFound
	case models.MailErrorRateLimit:
		status = http.StatusTooManyRequests
	case models.MailErrorConfiguration:
		status = http.StatusConflict
	case models.MailErrorUnavailable, models.MailErrorNetwork:
		status = http.StatusServiceUnavailable
	case models.MailErrorCancelled:
		status = http.StatusRequestTimeout
	}
	retryAfterSeconds := int64(0)
	if failure.RetryAfter > 0 {
		retryAfterSeconds = int64((failure.RetryAfter + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
	slog.Warn("mail operation failed", "method", c.Request.Method, "path", c.Request.URL.Path, "code", failure.Code, "error", err)
	c.JSON(status, models.ErrorResponse{Error: failure.Message, Code: failure.Code, Action: failure.Action, RetryAfterSeconds: retryAfterSeconds})
}

func (a *App) respondAccountMailError(c *gin.Context, account *models.Account, err error) {
	failure := normalizeMailError(err)
	if failure.Action != models.MailErrorActionNone {
		setAccountFailure(account, failure)
		now := time.Now()
		account.LastCheckedAt = &now
		if saveErr := a.saveAccount(c.Request.Context(), account); saveErr != nil {
			slog.Error("save account failure state", "accountID", account.ID, "error", saveErr)
		}
	}
	respondMailError(c, failure)
}
