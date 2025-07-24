package models

type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
	From User   `json:"from"`
}

type Chat struct {
	Id int `json:"id"`
}

type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type TelegramResponse struct {
	ChatId    int    `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type BotCommand struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	UserID  string   `json:"user_id"`
	ChatID  string   `json:"chat_id"`
}
