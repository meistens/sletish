package handlers

import (
	"net/http"
	"sletish/internal/bot"
	"sletish/internal/container"
	"sletish/internal/services"
)

func WebhookHandler(container *container.Container, botToken string) http.HandlerFunc {
	commandHandler := bot.NewHandler(container.AnimeService, container.UserService, container.Logger, botToken)

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

		ctx := r.Context()
		go func() {
			commandHandler.ProcessMessage(ctx, update)
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
