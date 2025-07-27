package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sletish/internal/models"
)

const telegramAPIURL = "https://api.telegram.org/bot"

func SendTelegramMessage(ctx context.Context, botToken string, chatId int, text string) error {
	response := models.TelegramResponse{
		ChatId:    chatId,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/sendMessage", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error: status %d", resp.StatusCode)
	}

	return nil
}

func ParseTelegramRequest(r *http.Request) (*models.Update, error) {
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, err
	}
	return &update, nil
}
