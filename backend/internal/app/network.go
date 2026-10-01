package app

import (
	"context"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

const (
	connectionTimeout = 25 * time.Second
	providerTimeout   = 90 * time.Second
	mailTimeout       = 5 * time.Minute
)

func mailDeadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(mailTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		return contextDeadline
	}
	return deadline
}

func directDialer() *net.Dialer {
	return &net.Dialer{Timeout: connectionTimeout, KeepAlive: 30 * time.Second}
}

type contextDialer struct {
	dialer proxy.Dialer
}

type dialResult struct {
	connection net.Conn
	err        error
}

func (d contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if contextual, ok := d.dialer.(proxy.ContextDialer); ok {
		return contextual.DialContext(ctx, network, address)
	}
	results := make(chan dialResult, 1)
	go d.dialAndReport(network, address, results)
	select {
	case result := <-results:
		return result.connection, result.err
	case <-ctx.Done():
		go closeLateConnection(results)
		return nil, ctx.Err()
	}
}

func (d contextDialer) dialAndReport(network, address string, results chan<- dialResult) {
	connection, err := d.dialer.Dial(network, address)
	results <- dialResult{connection: connection, err: err}
}

func closeLateConnection(results <-chan dialResult) {
	result := <-results
	if result.connection != nil {
		_ = result.connection.Close()
	}
}
