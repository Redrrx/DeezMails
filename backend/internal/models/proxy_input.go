package models

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"deezmails/internal/validation"
)

type ProxyInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (input ProxyInput) Build() (Proxy, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.Host = validation.NormalizeNetworkHost(strings.TrimSpace(input.Host))
	input.Username = strings.TrimSpace(input.Username)
	if input.Name == "" || len(input.Name) > 200 || validation.HasLineBreakOrNUL(input.Name) || !validation.ValidNetworkHost(input.Host) || input.Port < 1 || input.Port > 65535 {
		return Proxy{}, fmt.Errorf("name, host, and a valid port are required")
	}
	if len(input.Username) > 1024 || len(input.Password) > 64<<10 || validation.HasLineBreakOrNUL(input.Username) {
		return Proxy{}, fmt.Errorf("proxy credentials are too long or contain invalid characters")
	}
	if input.Type != "http" && input.Type != "https" && input.Type != "socks5" {
		return Proxy{}, fmt.Errorf("proxy type must be http, https, or socks5")
	}
	host := net.JoinHostPort(input.Host, fmt.Sprint(input.Port))
	return Proxy{
		Name: input.Name, Type: input.Type, Host: input.Host, Port: input.Port,
		Username: input.Username, Password: input.Password,
		URL: (&url.URL{Scheme: input.Type, Host: host}).String(),
	}, nil
}
