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

// SendTelegramMessage sends a plain text message to a Telegram chat.
//
// It wraps SendTelegramMessageWithKeyboard without any keyboard markup.
// Returns an error if sending the message fails.
func SendTelegramMessage(ctx context.Context, botToken string, chatId int, text string) error {
	return SendTelegramMessageWithKeyboard(ctx, botToken, chatId, text, nil)
}

// SendTelegramMessageWithKeyboard sends a text message to a Telegram chat,
// optionally including an inline keyboard for user interaction.
//
// Returns an error if marshaling the request, sending the HTTP request,
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

// EditTelegramMessage edits an existing message in a Telegram chat.
//
// Optionally updates the inline keyboard as well. Returns an error if marshaling
// the request, sending the HTTP request, or receiving a non-OK response from the
// Telegram API fails.
func EditTelegramMessage(ctx context.Context, botToken string, chatId int, messageId int, text string, keyboard *models.InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":    chatId,
		"message_id": messageId,
		"text":       text,
		"parse_mode": "HTML",
	}

	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal edit request: %w", err)
	}

	url := fmt.Sprintf("%s%s/editMessageText", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create edit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send edit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram edit message API error (status %d)", resp.StatusCode)
	}

	return nil
}

// EditTelegramMessageKeyboard updates only the inline keyboard of a message.
//
// Returns an error if marshaling the request, sending the HTTP request,
// or receiving a non-OK response from the Telegram API fails.
func EditTelegramMessageKeyboard(ctx context.Context, botToken string, chatId int, messageId int, keyboard *models.InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":      chatId,
		"message_id":   messageId,
		"reply_markup": keyboard,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal keyboard edit request: %w", err)
	}

	url := fmt.Sprintf("%s%s/editMessageReplyMarkup", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create keyboard edit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send keyboard edit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram edit keyboard API error (status %d)", resp.StatusCode)
	}

	return nil
}

// DeleteTelegramMessage deletes a message from a Telegram chat.
//
// Returns an error if marshaling the request, sending the HTTP request,
// or receiving a non-OK response from the Telegram API fails.
func DeleteTelegramMessage(ctx context.Context, botToken string, chatId int, messageId int) error {
	payload := map[string]interface{}{
		"chat_id":    chatId,
		"message_id": messageId,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %w", err)
	}

	url := fmt.Sprintf("%s%s/deleteMessage", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram delete message API error (status %d)", resp.StatusCode)
	}

	return nil
}

// AnswerCallbackQuery sends a response to a callback query triggered
// by a button in an inline keyboard.
//
// It can optionally show a popup notification (showAlert = true).
// Returns an error if marshaling the request, sending it, or getting
// a non-OK response from Telegram fails.
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

// SetBotCommands sets the list of available commands for the bot.
//
// These commands appear in Telegram's command menu. Returns an error if
// marshaling or sending the request fails.
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

// ParseTelegramRequest parses an incoming Telegram webhook HTTP request
// and returns the decoded Update object.
//
// Returns an error if decoding fails.
func ParseTelegramRequest(r *http.Request) (*models.Update, error) {
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, err
	}
	return &update, nil
}

// GetMessageID extracts the message ID from a Telegram message object.
//
// Currently returns 0 as a placeholder. You must update your models.Message
// struct to include the message_id field for this to work properly.
func GetMessageID(message *models.Message) int {
	return 0
}

// SendTypingAction sends a "typing..." action to a Telegram chat,
// indicating the bot is working or processing.
//
// Returns an error if marshaling or sending the request fails.
func SendTypingAction(ctx context.Context, botToken string, chatId int) error {
	payload := map[string]interface{}{
		"chat_id": chatId,
		"action":  "typing",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal typing action: %w", err)
	}

	url := fmt.Sprintf("%s%s/sendChatAction", telegramAPIURL, botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create typing action request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send typing action: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
