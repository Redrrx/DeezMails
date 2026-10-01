package app

import (
	"crypto/tls"
	"net/http"
	"time"

	"deezmails/internal/models"
)

func baseTransport(account models.Account) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = directDialer().DialContext
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.TLSHandshakeTimeout = connectionTimeout
	transport.ResponseHeaderTimeout = providerTimeout
	transport.MaxResponseHeaderBytes = 1 << 20
	transport.IdleConnTimeout = 90 * time.Second
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20

	if account.Proxy == nil {
		return transport, nil
	}
	u, err := proxyURL(account.Proxy)
	if err != nil {
		return nil, err
	}
	if account.Proxy.Type == "http" || account.Proxy.Type == "https" {
		transport.Proxy = http.ProxyURL(u)
		return transport, nil
	}
	dialer, err := accountDialer(account)
	if err != nil {
		return nil, err
	}
	transport.DialContext = (contextDialer{dialer: dialer}).DialContext
	return transport, nil
}

func providerHTTPClient(account models.Account) (*http.Client, error) {
	transport, err := baseTransport(account)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport, Timeout: providerTimeout, CheckRedirect: rejectOAuthRedirect}, nil
}

func rejectOAuthRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}
