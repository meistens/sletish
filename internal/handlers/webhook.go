package handlers

import (
	"net/http"
	"sletish/internal/logger"
	"sletish/internal/models"
	"sletish/internal/services"
	"strings"
)

func handleUpdate(botToken string, update *models.Update) {
	if update.Message.Text == "" {
		return
	}

	chatId := update.Message.Chat.Id
	text := update.Message.Text

	logger.Get().Infof("Received message: %s from chat: %d", text, chatId)

	if strings.HasPrefix(text, "/search") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			services.SendTelegramMessage(botToken, chatId, "Please provide a search query. Example: /search Naruto")
			return
		}

		query := strings.TrimSpace(parts[1])

		services.SendTelegramMessage(botToken, chatId, "Searching for anime...")

		searchResult, err := services.SearchAnime(query)
		if err != nil {
			logger.Get().Errorf("Error searching anime: %v", err)
			services.SendTelegramMessage(botToken, chatId, "Error occurred while searching. Please try again later.")
			return
		}

		message := services.FormatAnimeMessage(searchResult.Data)
		services.SendTelegramMessage(botToken, chatId, message)

	} else if text == "/start" {
		welcomeMessage := `<b>Welcome to Anime Search Bot!</b>

This bot helps you search for anime using the Jikan API (MyAnimeList).

<b>Available Commands:</b>
/search <query> - Search for anime

<b>Example:</b>
/search Naruto
/search Attack on Titan

Happy searching!`

		services.SendTelegramMessage(botToken, chatId, welcomeMessage)
	}
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

		go handleUpdate(botToken, update)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
