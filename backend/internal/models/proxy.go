package models

import (
	"fmt"
	"net"
	"net/url"
	"time"
)

type Proxy struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	Type      string    `json:"type" gorm:"not null;default:http"`
	Host      string    `json:"host" gorm:"not null;default:''"`
	Port      int       `json:"port" gorm:"not null;default:0"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	URL       string    `json:"-" gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`
}

func (proxy Proxy) EndpointURL() string {
	endpoint := &url.URL{Scheme: proxy.Type, Host: net.JoinHostPort(proxy.Host, fmt.Sprint(proxy.Port))}
	if proxy.Username != "" || proxy.Password != "" {
		endpoint.User = url.UserPassword(proxy.Username, proxy.Password)
	}
	return endpoint.String()
}
