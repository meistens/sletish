package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sletish/internal/models"

	"github.com/sirupsen/logrus"
)

const tgAPIURL = "https://api.telegram.org/bot"

type Bot struct {
	Token  string
	Logger *logrus.Logger
}

func NewBot(token string, logger *logrus.Logger) *Bot {
	return &Bot{
		Token:  token,
		Logger: logger,
	}
}

func (b *Bot) SendMessage(chatID int, text string) error {
	response := models.TelegramResponse{
		ChatId:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	url := fmt.Sprintf("%s%s/sendMessage", tgAPIURL, b.Token)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TG API returned status %d", resp.StatusCode)
	}

	return nil
}

func (b *Bot) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	update, err := b.parseRequest(r)
	if err != nil {
		log.Printf("Error parsing request: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	go b.HandleUpdate(update)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (b *Bot) parseRequest(r *http.Request) (*models.Update, error) {
	var update models.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, fmt.Errorf("could not decode incoming update: %w", err)
	}

	return &update, nil
}
