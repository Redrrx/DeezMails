package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"deezmails/internal/models"
)

func proxyURL(p *models.Proxy) (*url.URL, error) {
	if p == nil {
		return nil, nil
	}
	u, err := url.Parse(p.EndpointURL())
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid proxy configuration")
	}
	return u, nil
}

func accountDialer(account models.Account) (proxy.Dialer, error) {
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
		return &httpConnectDialer{proxyURL: u, forward: forward}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", account.Proxy.Type)
	}
}

type httpConnectDialer struct {
	proxyURL *url.URL
	forward  proxy.Dialer
}

func (d *httpConnectDialer) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	return d.DialContext(ctx, network, address)
}

func (d *httpConnectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("HTTP CONNECT supports only TCP")
	}
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	connection, err := (contextDialer{dialer: d.forward}).DialContext(ctx, "tcp", d.proxyURL.Host)
	if err != nil {
		return nil, err
	}
	closeOnCancel := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer closeOnCancel()
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			connection.Close()
			return nil, err
		}
	}
	if d.proxyURL.Scheme == "https" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: d.proxyURL.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			connection.Close()
			return nil, err
		}
		connection = tlsConnection
	}
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	if d.proxyURL.User != nil {
		password, _ := d.proxyURL.User.Password()
		encoded := base64.StdEncoding.EncodeToString([]byte(d.proxyURL.User.Username() + ":" + password))
		request.Header.Set("Proxy-Authorization", "Basic "+encoded)
	}
	if err := request.Write(connection); err != nil {
		connection.Close()
		return nil, err
	}
	limited := &proxyHeaderReader{reader: connection, remaining: 64 << 10}
	reader := bufio.NewReaderSize(limited, 32<<10)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		connection.Close()
		return nil, err
	}
	limited.unlimited = true
	if response.StatusCode != http.StatusOK {
		io.CopyN(io.Discard, response.Body, 4096)
		response.Body.Close()
		connection.Close()
		return nil, fmt.Errorf("HTTP proxy refused CONNECT with status %d", response.StatusCode)
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		connection.Close()
		return nil, err
	}
	if reader.Buffered() != 0 {
		return &bufferedConnection{Conn: connection, reader: reader}, nil
	}
	return connection, nil
}

type bufferedConnection struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConnection) Read(target []byte) (int, error) {
	return c.reader.Read(target)
}

type proxyHeaderReader struct {
	reader    io.Reader
	remaining int64
	unlimited bool
}

func (r *proxyHeaderReader) Read(target []byte) (int, error) {
	if r.unlimited {
		return r.reader.Read(target)
	}
	if r.remaining <= 0 {
		return 0, fmt.Errorf("HTTP proxy response headers exceed 65536 bytes")
	}
	if int64(len(target)) > r.remaining {
		target = target[:r.remaining]
	}
	read, err := r.reader.Read(target)
	r.remaining -= int64(read)
	return read, err
}
