package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/api"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/auth"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/config"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/orchestrator"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/repository"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/telemetry"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/pkg/k8sclient"
)

func main() {
	// Logger
	logger, _ := zap.NewProduction()
	if os.Getenv("APP_ENV") == "development" {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Telemetry
	shutdown, err := telemetry.InitTracer(cfg.OtelEndpoint, "idp-control-plane")
	if err != nil {
		logger.Warn("failed to init tracer", zap.Error(err))
	} else {
		defer shutdown(context.Background())
	}

	// Database
	db, err := repository.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// K8s client
	k8sClient, err := k8sclient.New(cfg.KubeConfig, cfg.K8sInCluster)
	if err != nil {
		logger.Fatal("failed to create k8s client", zap.Error(err))
	}

	// Services
	authSvc := auth.NewService(cfg.JWTSecret, db)
	orch := orchestrator.New(k8sClient, cfg.MLEngineURL, logger)

	// API
	handler := api.NewHandler(api.Config{
		Logger:       logger,
		Auth:         authSvc,
		Orchestrator: orch,
		DB:           db,
		MLEngineURL:  cfg.MLEngineURL,
	})

	router := api.NewRouter(handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting",
			zap.Int("port", cfg.Port),
			zap.String("env", cfg.AppEnv),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-done
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}
