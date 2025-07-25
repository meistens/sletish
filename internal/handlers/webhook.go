package handlers

import (
	"net/http"
	"sletish/internal/bot"
	"sletish/internal/logger"
	"sletish/internal/models"
	"sletish/internal/services"

	"github.com/redis/go-redis/v9"
)

func handleUpdate(update *models.Update) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // env later
	})

	commandHandler := bot.NewHandler(logger.Get(), redisClient)
	commandHandler.ProcessMessage(update)
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
