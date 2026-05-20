package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/companyofcreators/chat-service/internal/app"
	"github.com/companyofcreators/chat-service/internal/config"
	httpInterface "github.com/companyofcreators/chat-service/internal/interfaces/http"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialize container with all dependencies.
	container, err := app.NewContainer(ctx, cfg)
	if err != nil {
		panic("failed to initialize container: " + err.Error())
	}
	defer container.Shutdown(ctx)

	logger := container.Logger

	// Build router.
	router := httpInterface.NewRouter(
		container.HTTPHandler,
		container.WSHandler.Upgrade,
		logger,
	)

	// Start the WebSocket hub in the background.
	go func() {
		if err := container.Hub.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("hub run error", "error", err)
		}
	}()

	// Create HTTP server.
	srv := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	go func() {
		logger.Info("starting chat service", "address", cfg.HTTPAddress)
		logger.Info("websocket endpoint", "path", "ws://"+cfg.HTTPAddress+"/ws")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("received shutdown signal", "signal", sig.String())

	// Graceful shutdown with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("chat service stopped")
}
