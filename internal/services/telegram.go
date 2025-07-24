package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sletish/internal/models"
)

const telegramAPIURL = "https://api.telegram.org/bot"

func SendTelegramMessage(botToken string, chatId int, text string) error {
	response := models.TelegramResponse{
		ChatId: chatId,
		Text:   text,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/sendMessage", telegramAPIURL, botToken)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func ParseTelegramRequest(r *http.Request) (*models.Update, error) {
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, err
	}
	return &update, nil
}
