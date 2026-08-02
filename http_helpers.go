package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Next()
	}
}

func limitRequestBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func newAccessToken() string {
	token := env("DEEZMAILS_ACCESS_TOKEN", "")
	if len(token) < 32 {
		panic("DEEZMAILS_ACCESS_TOKEN must be at least 32 characters")
	}
	return token
}

func (a *App) requireAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		value := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if len(value) != len(a.accessToken) || subtle.ConstantTimeCompare([]byte(value), []byte(a.accessToken)) != 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "a valid Bearer access token is required"})
			return
		}
		c.Next()
	}
}

func validProvider(provider string) bool {
	return provider == "gmail" || provider == "microsoft" || provider == "imap" || provider == "pop3"
}

func usesOAuth(provider string) bool { return provider == "gmail" || provider == "microsoft" }

func validIncoming(input AccountInput) bool {
	if input.Provider != "imap" && input.Provider != "pop3" {
		return true
	}
	return input.IncomingHost != "" && input.IncomingPort > 0 && input.IncomingPort < 65536 && (input.TLSMode == "implicit_tls" || input.TLSMode == "starttls" || input.TLSMode == "none") && input.Password != ""
}

func positive(text string, fallback int) int {
	value, err := strconv.Atoi(text)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func randomHex(bytes int) string {
	data := make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func bad(c *gin.Context, message string) {
	c.JSON(400, gin.H{"error": message})
}

func notFound(c *gin.Context) {
	c.JSON(404, gin.H{"error": "not found"})
}
