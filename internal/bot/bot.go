package bot

import (
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
