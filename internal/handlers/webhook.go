package handlers

import (
	"context"
	"net/http"
	"sletish/internal/bot"
	"sletish/internal/cache"
	"sletish/internal/logger"
	"sletish/internal/models"
	"sletish/internal/services"
)

func handleUpdate(update *models.Update) {
	ctx := context.Background()
	redisClient := cache.Get()
	commandHandler := bot.NewHandler(logger.Get(), redisClient)
	commandHandler.ProcessMessage(ctx, update)
}

func WebhookHandler(botToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		update, err := services.ParseTelegramRequest(r)
		if err != nil {
			logger.Get().Errorf("Error parsing request: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		go handleUpdate(update)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
