package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"deezmails/internal/models"

	"golang.org/x/oauth2"
)

type responseLimitTransport struct {
	base     http.RoundTripper
	maxBytes int64
	provider string
}

func (transport responseLimitTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := validateProviderAPIURL(transport.provider, request.URL.String()); err != nil {
		return nil, err
	}
	response, err := transport.base.RoundTrip(request)
	if response != nil {
		response.Body = http.MaxBytesReader(nil, response.Body, transport.maxBytes)
	}
	return response, err
}

func providerClient(account models.Account, token *oauth2.Token, maxBytes int64) (*http.Client, error) {
	base, err := providerHTTPClient(account)
	if err != nil {
		return nil, err
	}
	provider := account.Provider
	return &http.Client{
		Timeout: providerTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("provider returned too many redirects")
			}
			return validateProviderAPIURL(provider, request.URL.String())
		},
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base:   responseLimitTransport{base: base.Transport, maxBytes: maxBytes, provider: provider},
		},
	}, nil
}

func validateProviderAPIURL(provider, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || (parsed.Port() != "" && parsed.Port() != "443") {
		return fmt.Errorf("provider returned an invalid API URL")
	}
	expectedHost := "graph.microsoft.com"
	if provider == models.ProviderGmail {
		expectedHost = "gmail.googleapis.com"
	}
	if !strings.EqualFold(parsed.Hostname(), expectedHost) {
		return fmt.Errorf("provider attempted to use an untrusted host")
	}
	return nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	return body, nil
}

func (a *App) microsoftGet(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := readLimited(response.Body, a.config.MaxMessageBytes)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 300 {
		var payload microsoftErrorResponse
		_ = json.Unmarshal(body, &payload)
		codes := make([]string, 0, 2)
		for detail := &payload.Error; detail != nil; detail = detail.InnerError {
			if detail.Code != "" {
				codes = append(codes, detail.Code)
			}
		}
		return nil, &microsoftHTTPError{Status: response.StatusCode, Codes: codes, Message: payload.Error.Message, RetryAfter: response.Header.Get("Retry-After")}
	}
	return body, nil
}

func (a *App) microsoftJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	body, err := a.microsoftGet(ctx, client, endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
