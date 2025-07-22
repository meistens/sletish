package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
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
