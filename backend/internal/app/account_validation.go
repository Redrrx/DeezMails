package app

import (
	"fmt"
	"net/mail"

	"deezmails/internal/models"
	"deezmails/internal/validation"
)

func validProvider(provider string) bool {
	return provider == models.ProviderGmail || provider == models.ProviderMicrosoft || provider == models.ProviderIMAP || provider == models.ProviderPOP3
}

func usesOAuth(provider string) bool {
	return provider == models.ProviderGmail || provider == models.ProviderMicrosoft
}

func (a *App) validIncoming(input models.AccountInput) bool {
	if input.Provider != models.ProviderIMAP && input.Provider != models.ProviderPOP3 {
		return true
	}
	validTLS := input.TLSMode == models.TLSModeImplicit || input.TLSMode == models.TLSModeStartTLS || (input.TLSMode == models.TLSModeNone && a.config.AllowInsecureMailAuth)
	return validation.ValidNetworkHost(input.IncomingHost) && input.IncomingPort > 0 && input.IncomingPort < 65536 && validTLS && input.Password != "" && len(input.Password) <= 64<<10 && !validation.HasLineBreakOrNUL(input.Email) && !validation.HasLineBreakOrNUL(input.Password)
}

func (a *App) validateMailTransport(account models.Account) error {
	if account.Provider != models.ProviderIMAP && account.Provider != models.ProviderPOP3 {
		return nil
	}
	if !validation.ValidNetworkHost(account.IncomingHost) || account.IncomingPort < 1 || account.IncomingPort > 65535 || account.MailboxPassword == "" || validation.HasLineBreakOrNUL(account.Email) || validation.HasLineBreakOrNUL(account.MailboxPassword) {
		return &mailFailure{Code: models.MailErrorConfiguration, Action: models.MailErrorActionEditAccount, Message: "Direct-mail account configuration is invalid.", Cause: fmt.Errorf("direct-mail account configuration is invalid")}
	}
	switch account.TLSMode {
	case models.TLSModeImplicit, models.TLSModeStartTLS:
		return nil
	case models.TLSModeNone:
		if !a.config.AllowInsecureMailAuth {
			return &mailFailure{Code: models.MailErrorConfiguration, Action: models.MailErrorActionEditAccount, Message: "Plaintext mailbox authentication is disabled. Choose TLS.", Cause: fmt.Errorf("plaintext IMAP/POP3 authentication is disabled")}
		}
		return nil
	default:
		return &mailFailure{Code: models.MailErrorConfiguration, Action: models.MailErrorActionEditAccount, Message: "Direct-mail security mode is unsupported.", Cause: fmt.Errorf("unsupported direct-mail TLS mode")}
	}
}

func validMailboxAddress(value string) bool {
	if value == "" || len(value) > 320 || validation.HasLineBreakOrNUL(value) {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}
