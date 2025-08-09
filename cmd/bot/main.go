package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sletish/internal/container"
	"sletish/internal/handlers"
	"sletish/internal/logger"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	logger.Init()
	log := logger.Get()

	err := godotenv.Load()
	if err != nil {
		log.Info("No .env file found, using system environment variables")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is required. Set it in .env file or as environment variable")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	container, err := container.New(ctx)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize container")
	}
	defer container.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handlers.WebhookHandler(container, botToken))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Infof("Bot starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed to start")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	sdCtx, sdCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sdCancel()
	if err := server.Shutdown(sdCtx); err != nil {
		log.WithError(err).Error("Server forced to shutdown")
	}

	log.Info("Server exited")
}
