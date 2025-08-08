package models

// Update represents an incoming update from Telegram,
// which may contain either a message or a callback query.
type Update struct {
	UpdateId      int            `json:"update_id"`
	Message       Message        `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message represents a standard text message sent in a chat.
type Message struct {
	MessageId int    `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
}

// Chat represents a Telegram chat, which may be a private chat, group, etc.
type Chat struct {
	Id int `json:"id"`
}

// User represents a Telegram user or bot who sent the message or query.
type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// CallbackQuery represents a callback query triggered by
// an inline keyboard button.
type CallbackQuery struct {
	Id      string  `json:"id"`
	From    User    `json:"from"`
	Message Message `json:"message"`
	Data    string  `json:"data"`
}

// InlineKeyboardMarkup defines an inline keyboard that appears
// alongside a message.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton represents a single button in an inline keyboard.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

// TelegramResponse represents the payload sent to Telegram's sendMessage API.
type TelegramResponse struct {
	ChatId      int                   `json:"chat_id"`
	Text        string                `json:"text"`
	ParseMode   string                `json:"parse_mode,omitempty"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// BotCommandMenu defines a command and description for Telegram's bot command menu.
type BotCommandMenu struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// BotCommand represents a parsed user command input, used internally
// to handle user commands and their arguments.
type BotCommand struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	UserID  string   `json:"user_id"`
	ChatID  string   `json:"chat_id"`
}

// CallbackData defines the structure of data attached to inline buttons
// to facilitate various types of user interaction, including pagination.
type CallbackData struct {
	Action  string `json:"action"`
	AnimeID string `json:"anime_id,omitempty"`
	Status  string `json:"status,omitempty"`
	Page    int    `json:"page,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// AnswerCallbackQuery represents a request to respond to a callback query.
// It is used to send a notification or alert back to the user after an inline button is pressed.
type AnswerCallbackQuery struct {
	CallbackQueryId string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}
