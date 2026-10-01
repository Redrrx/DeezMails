package validation

import (
	"net"
	"net/url"
	"strings"
)

func HasLineBreakOrNUL(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

func ValidNetworkHost(host string) bool {
	host = NormalizeNetworkHost(strings.TrimSpace(host))
	if host == "" || len(host) > 253 || HasLineBreakOrNUL(host) || strings.ContainsAny(host, "/\\@?#") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func NormalizeNetworkHost(host string) string {
	if parsed, err := url.Parse("//" + host); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return strings.Trim(host, "[]")
}

func IsLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
