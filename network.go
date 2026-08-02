package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/magisterquis/connectproxy"
	"golang.org/x/net/proxy"
)

const (
	connectionTimeout = 25 * time.Second
	providerTimeout   = 90 * time.Second
	mailTimeout       = 5 * time.Minute
	defaultMaxMessage = 64 << 20
)

func maxMessageBytes() int64 {
	value, err := strconv.ParseInt(env("MAX_MESSAGE_BYTES", ""), 10, 64)
	if err == nil && value > 0 {
		return value
	}
	return defaultMaxMessage
}

func directDialer() *net.Dialer {
	return &net.Dialer{Timeout: connectionTimeout, KeepAlive: 30 * time.Second}
}

func proxyURL(p *Proxy) (*url.URL, error) {
	if p == nil {
		return nil, nil
	}
	u, err := url.Parse(p.endpointURL())
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid proxy configuration")
	}
	return u, nil
}

func accountDialer(account Account) (proxy.Dialer, error) {
	forward := directDialer()
	if account.Proxy == nil {
		return forward, nil
	}
	u, err := proxyURL(account.Proxy)
	if err != nil {
		return nil, err
	}
	switch account.Proxy.Type {
	case "socks5":
		var auth *proxy.Auth
		if account.Proxy.Username != "" || account.Proxy.Password != "" {
			auth = &proxy.Auth{User: account.Proxy.Username, Password: account.Proxy.Password}
		}
		return proxy.SOCKS5("tcp", u.Host, auth, forward)
	case "http", "https":
		return connectproxy.NewWithConfig(u, forward, &connectproxy.Config{DialTimeout: connectionTimeout})
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", account.Proxy.Type)
	}
}

func dialContext(dialer proxy.Dialer) func(context.Context, string, string) (net.Conn, error) {
	if contextual, ok := dialer.(proxy.ContextDialer); ok {
		return contextual.DialContext
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		type result struct {
			conn net.Conn
			err  error
		}
		resultCh := make(chan result, 1)
		go func() {
			conn, err := dialer.Dial(network, address)
			resultCh <- result{conn: conn, err: err}
		}()
		select {
		case result := <-resultCh:
			return result.conn, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func baseTransport(account Account) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = directDialer().DialContext
	transport.TLSHandshakeTimeout = connectionTimeout
	transport.ResponseHeaderTimeout = providerTimeout
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
	transport.DialContext = dialContext(dialer)
	return transport, nil
}

func providerHTTPClient(account Account) (*http.Client, error) {
	transport, err := baseTransport(account)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport, Timeout: providerTimeout}, nil
}
