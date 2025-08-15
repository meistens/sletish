package handlers

import (
	"context"
	"net/http"
	"sletish/internal/bot"
	"sletish/internal/container"
	"sletish/internal/services"
	"time"
)

func WebhookHandler(container *container.Container, botToken string) http.HandlerFunc {
	// set bot token for reminder service
	container.ReminderService.SetBotToken(botToken)

	commandHandler := bot.NewHandler(
		container.AnimeService,
		container.UserService,
		container.ReminderService, // ORDER OF DEPS MATTER, BEFORE YOU END UP DEBUGGING A NON-ISSUE!!!!
		container.Logger,
		botToken,
	)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		update, err := services.ParseTelegramRequest(r)
		if err != nil {
			container.Logger.WithError(err).Error("Error parsing request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		go func() {
			defer cancel()
			commandHandler.ProcessMessage(ctx, update)
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
