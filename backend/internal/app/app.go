package app

import (
	"context"
	"crypto/cipher"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deezmails/internal/config"
	"deezmails/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type App struct {
	db          *gorm.DB
	credentials cipher.AEAD
	jobs        *jobService
	config      config.Config
}

func Run() error {
	appConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := storage.Open(ctx, appConfig)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("access database connection pool: %w", err)
	}
	defer sqlDB.Close()
	startupContext, cancelStartup := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelStartup()
	if err := storage.Migrate(startupContext, db); err != nil {
		return err
	}
	credentials, err := newCredentialCipher(appConfig.CredentialKey)
	if err != nil {
		return err
	}
	application := &App{db: db, credentials: credentials, config: appConfig}
	if err := application.prepareCredentials(startupContext); err != nil {
		return err
	}
	cancelStartup()

	if err := application.startJobs(ctx); err != nil {
		return err
	}
	server := &http.Server{
		Addr:                appConfig.ListenAddress,
		Handler:             application.router(),
		ReadHeaderTimeout:   10 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        5 * time.Minute,
		IdleTimeout:         90 * time.Second,
		MaxHeaderBytes:      1 << 20,
		MaxHeaderValueCount: 128,
	}
	slog.Info("server configured", "address", appConfig.ListenAddress, "authRequired", true)
	serveErr := serve(ctx, server)
	stop()
	workerContext, cancelWorkers := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelWorkers()
	if err := application.jobs.wait(workerContext); err != nil && serveErr == nil {
		return fmt.Errorf("wait for background jobs to stop: %w", err)
	}
	return serveErr
}

func serve(ctx context.Context, server *http.Server) error {
	serverErrors := make(chan error, 1)
	go listenAndReport(server, serverErrors)
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		return nil
	}
}

func listenAndReport(server *http.Server, results chan<- error) {
	results <- server.ListenAndServe()
}
