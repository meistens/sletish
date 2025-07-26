package main

import (
	"context"
	"net/http"
	"os"
	"sletish/internal/database"
	"sletish/internal/handlers"
	"sletish/internal/logger"

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

	// database
	database.MustInit(context.Background())
	defer database.Close()

	http.HandleFunc("/webhook", handlers.WebhookHandler(botToken))

	log.Infof("Bot starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
