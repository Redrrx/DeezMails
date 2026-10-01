package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	contentSecurityPolicy := "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	if c.Request.URL.Path == "/" || c.Request.URL.Path == "/favicon.svg" || strings.HasPrefix(c.Request.URL.Path, "/assets/") {
		contentSecurityPolicy = "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'"
	} else if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
		contentSecurityPolicy = "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"
	}
	c.Header("Content-Security-Policy", contentSecurityPolicy)
	c.Next()
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Next()
}

func limitRequestBody(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	c.Next()
}

func recoverRequest(c *gin.Context, recovered any) {
	slog.Error("panic recovered while serving request", "method", c.Request.Method, "path", c.Request.URL.Path, "panicType", fmt.Sprintf("%T", recovered))
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func bindJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func (a *App) requireAccessToken(c *gin.Context) {
	value := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if len(value) != len(a.config.AccessToken) || subtle.ConstantTimeCompare([]byte(value), []byte(a.config.AccessToken)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":  "access_token_required",
			"error": "a valid Bearer access token is required",
		})
		return
	}
	c.Next()
}

func (a *App) requestDB(c *gin.Context) *gorm.DB {
	return a.db.WithContext(c.Request.Context())
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
}

func internalError(c *gin.Context, publicMessage string, err error) {
	slog.Error(publicMessage, "method", c.Request.Method, "path", c.Request.URL.Path, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": publicMessage})
}

func databaseWriteError(c *gin.Context, action string, err error) {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		c.JSON(http.StatusConflict, gin.H{"error": "a record with the same unique value already exists"})
		return
	}
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		badRequest(c, "the referenced record does not exist")
		return
	}
	internalError(c, action, err)
}

func handleLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	internalError(c, "could not load record", err)
}
