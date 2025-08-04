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

// SendTelegramMessage sends a plain text message to a Telegram chat by
// calling SendTelegramMessageWithKeyboard.
// It returns an error if sending the message fails.
func SendTelegramMessage(ctx context.Context, botToken string, chatId int, text string) error {
	return SendTelegramMessageWithKeyboard(ctx, botToken, chatId, text, nil)
}

// SendTelegramMessageWithKeyboard sends a text message to a Telegram chat,
// optionally including an inline keyboard for user interaction.
// It returns an error if marshaling the request, sending the HTTP request,
// or receiving a non-OK response from the Telegram API fails.
func SendTelegramMessageWithKeyboard(ctx context.Context, botToken string, chatId int, text string, keyboard *models.InlineKeyboardMarkup) error {
	response := models.TelegramResponse{
		ChatId:      chatId,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s/sendMessage", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error (status %d)", resp.StatusCode)
	}

	return nil
}

// AnswerCallbackQuery sends a response to a Telegram callback query triggered by
// an inline keyboard button. It can optionally display a notification or alert to the user(bool value set).
// It returns an error if either marshaling the request, sending the HTTP request,
// or receiving a non-OK response from the Telegram API fails.
func AnswerCallbackQuery(ctx context.Context, botToken string, callbackQueryId string, text string, showAlert bool) error {
	response := models.AnswerCallbackQuery{
		CallbackQueryId: callbackQueryId,
		Text:            text,
		ShowAlert:       showAlert,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal callback answer: %w", err)
	}

	url := fmt.Sprintf("%s%s/answerCallbackQuery", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create callback answer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send callback answer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram callback answer API error (status %d)", resp.StatusCode)
	}

	return nil
}

// SetBotCommands configures the list of commands for the Telegram bot,
// which appear in the bot's command menu to assist users in interacting with the bot.
// It returns an error if either marshaling the commands, sending the HTTP request,
// or receiving a non-OK response from the Telegram API fails.
func SetBotCommands(ctx context.Context, botToken string) error {
	// NOTE: revisit after adding new commands
	commands := []models.BotCommandMenu{
		{Command: "start", Description: "🚀 Start the bot and see welcome message"},
		{Command: "search", Description: "🔍 Search for anime by name"},
		{Command: "add", Description: "➕ Add anime to your list"},
		{Command: "list", Description: "📋 View your anime list"},
		{Command: "update", Description: "🔄 Update anime status in your list"},
		{Command: "remove", Description: "🗑 Remove anime from your list"},
		{Command: "profile", Description: "👤 View your profile and stats"},
		{Command: "help", Description: "❓ Show help and available commands"},
	}

	payload := map[string]interface{}{
		"commands": commands,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal commands: %w", err)
	}

	url := fmt.Sprintf("%s%s/setMyCommands", telegramAPIURL, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create set commands request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set bot commands: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set bot commands API error (status %d)", resp.StatusCode)
	}

	return nil
}

// ParseTelegramRequest reads and decodes a Telegram update from the HTTP request body.
// It returns the parsed Update object or an error if decoding fails.
func ParseTelegramRequest(r *http.Request) (*models.Update, error) {
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, err
	}
	return &update, nil
}
