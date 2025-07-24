package bot

import (
	"os"
	"sletish/internal/models"
	"sletish/internal/services"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type BotCommand struct {
	Command string
	Args    []string
	UserID  string
	ChatID  string
}

type Handler struct {
	animeService *services.Client
	logger       *logrus.Logger
	botToken     string
}

func NewHandler(logger *logrus.Logger) *Handler {
	return &Handler{
		animeService: services.NewClient(),
		logger:       logger,
		botToken:     os.Getenv("BOT_TOKEN"),
	}
}

func (h *Handler) ProcessMessage(update *models.Update) {
	if update.Message.Text == "" {
		return
	}

	userID := strconv.Itoa(update.Message.From.Id)
	chatID := strconv.Itoa(update.Message.Chat.Id)
	text := strings.TrimSpace(update.Message.Text)

	command := h.parseCommand(text, userID, chatID)
	h.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"command": command.Command,
		"args":    command.Args,
	}).Info("Processing command")

	switch command.Command {
	case "/start":
		h.handleStart(command)
	case "/search":
		h.handleSearch(command)
	case "/help":
		h.handleHelp(command)
	default:
		h.sendMessage(command.ChatID, "Unknown command. Use /start to see available commands")
	}
}

func (h *Handler) parseCommand(text, userID, chatID string) BotCommand {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return BotCommand{UserID: userID, ChatID: chatID}
	}

	return BotCommand{
		Command: parts[0],
		Args:    parts[1:],
		UserID:  userID,
		ChatID:  chatID,
	}
}

func (h *Handler) handleStart(cmd BotCommand) {
	welcomeMessage := `Welcome to My Media Search Bot!

	/search name_of_anime

	P.s: anime is the only command working at the moment`

	h.sendMessage(cmd.ChatID, welcomeMessage)
}

func (h *Handler) handleSearch(cmd BotCommand) {
	if len(cmd.Args) == 0 {
		h.sendMessage(cmd.ChatID, "Please provide an anime name to search. Example: /search Naruto")
		return
	}

	query := strings.Join(cmd.Args, " ")

	h.sendMessage(cmd.ChatID, "Searching for anime...")

	searchResult, err := h.animeService.SearchAnime(query)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search anime")
		h.sendMessage(cmd.ChatID, "Error occurred while searching. Please try again later.")
		return
	}

	message := services.FormatAnimeMessage(searchResult.Data)
	h.sendMessage(cmd.ChatID, message)
}

func (h *Handler) handleHelp(cmd BotCommand) {
	h.handleStart(cmd)
}

func (h *Handler) sendMessage(chatID, text string) {
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		h.logger.WithError(err).Error("Invalid chat ID")
		return
	}

	if err := services.SendTelegramMessage(h.botToken, chatIDInt, text); err != nil {
		h.logger.WithError(err).Error("Failed to send message")
	}
}
