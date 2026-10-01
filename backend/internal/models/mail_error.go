package models

type MailErrorCode string

const (
	MailErrorAuthentication   MailErrorCode = "authentication_failed"
	MailErrorPermission       MailErrorCode = "permission_denied"
	MailErrorPolicy           MailErrorCode = "organization_policy"
	MailErrorQuota            MailErrorCode = "quota_exceeded"
	MailErrorRateLimit        MailErrorCode = "rate_limited"
	MailErrorUnavailable      MailErrorCode = "provider_unavailable"
	MailErrorNotFound         MailErrorCode = "not_found"
	MailErrorConfiguration    MailErrorCode = "invalid_configuration"
	MailErrorTLS              MailErrorCode = "tls_failed"
	MailErrorNetwork          MailErrorCode = "network_failed"
	MailErrorProtocol         MailErrorCode = "protocol_failed"
	MailErrorResponseTooLarge MailErrorCode = "response_too_large"
	MailErrorCancelled        MailErrorCode = "cancelled"
	MailErrorInternal         MailErrorCode = "internal_error"
)

type MailErrorAction string

const (
	MailErrorActionNone         MailErrorAction = "none"
	MailErrorActionRetry        MailErrorAction = "retry"
	MailErrorActionReconnect    MailErrorAction = "reconnect"
	MailErrorActionEditAccount  MailErrorAction = "edit_account"
	MailErrorActionContactAdmin MailErrorAction = "contact_admin"
)

type ErrorResponse struct {
	Error             string          `json:"error"`
	Code              MailErrorCode   `json:"code"`
	Action            MailErrorAction `json:"action"`
	RetryAfterSeconds int64           `json:"retryAfterSeconds,omitempty"`
}
