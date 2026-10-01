package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) health(c *gin.Context) {
	sqlDB, err := a.db.DB()
	if err == nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "mode": runtimeMode})
}
