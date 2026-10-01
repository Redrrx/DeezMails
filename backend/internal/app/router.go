package app

import (
	"io"
	"net/http"
	"path/filepath"

	_ "deezmails/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const runtimeMode = "production"

func (a *App) router() *gin.Engine {
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipQueryString: true}))
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, recoverRequest))
	r.Use(securityHeaders)
	r.GET("/health", a.health)
	r.GET("/api/runtime", a.runtimeInfo)
	routes := r.Group("/api", a.requireAccessToken)
	routes.Use(noStore)
	routes.Use(limitRequestBody)
	routes.POST("/proxies", a.createProxy)
	routes.GET("/proxies", a.listProxies)
	routes.PUT("/proxies/:id", a.updateProxy)
	routes.DELETE("/proxies/:id", a.deleteProxy)
	routes.POST("/accounts", a.createAccount)
	routes.GET("/accounts", a.listAccounts)
	routes.GET("/accounts/:id", a.getAccount)
	routes.PUT("/accounts/:id", a.updateAccount)
	routes.DELETE("/accounts/:id", a.deleteAccount)
	routes.POST("/accounts/:id/reconnect", a.reconnect)
	r.GET("/oauth/callback", noStore, a.oauthCallback)
	routes.POST("/accounts/:id/sync", a.sync)
	routes.GET("/accounts/:id/folders", a.listFolders)
	routes.GET("/accounts/:id/emails", a.listEmails)
	routes.GET("/accounts/:id/emails/:messageID", a.getEmail)
	routes.GET("/jobs", a.listJobs)
	routes.GET("/jobs/summary", a.listJobSummaries)
	routes.DELETE("/jobs", a.clearJobs)
	routes.GET("/jobs/:id", a.getJob)
	routes.POST("/jobs/:id/retry", a.retryJob)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	if a.config.FrontendDistDir != "" {
		r.Static("/assets", filepath.Join(a.config.FrontendDistDir, "assets"))
		r.StaticFile("/favicon.svg", filepath.Join(a.config.FrontendDistDir, "favicon.svg"))
		r.GET("/", a.serveFrontendIndex)
	}
	return r
}

func (a *App) runtimeInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"mode": runtimeMode, "authRequired": true})
}

func (a *App) serveFrontendIndex(c *gin.Context) {
	c.File(filepath.Join(a.config.FrontendDistDir, "index.html"))
}
