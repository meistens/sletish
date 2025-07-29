package bot

import (
	"context"
	"os"
	"sletish/internal/models"
	"sletish/internal/services"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	userService  *services.UserService
	logger       *logrus.Logger
	botToken     string
}

func NewHandler(logger *logrus.Logger, redisClient *redis.Client) *Handler {
	config := &services.ClientConfig{
		BaseURL:    "https://api.jikan.moe/v4",
		Timeout:    30 * time.Second,
		RateLimit:  1 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		UserAgent:  "AnimeTrackerBot/1.0",
		Logger:     logger,
		Redis:      redisClient,
	}
	return &Handler{
		animeService: services.NewClientWithConfig(config),
		userService:  services.NewUserService(),
		logger:       logger,
		botToken:     os.Getenv("BOT_TOKEN"),
	}
}

func (h *Handler) ProcessMessage(ctx context.Context, update *models.Update) {
	if update.Message.Text == "" {
		return
	}

	username := update.Message.From.Username
	userID := strconv.Itoa(update.Message.From.Id)
	chatID := strconv.Itoa(update.Message.Chat.Id)

	// Always try to ensure user exists, but don't block commands if it fails
	go func() {
		if err := h.userService.EnsureUserExists(userID, username); err != nil {
			h.logger.WithError(err).Error("failed to ensure user exists")
		}
	}()

	text := strings.TrimSpace(update.Message.Text)
	command := h.parseCommand(text, userID, chatID)

	h.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"command": command.Command,
		"args":    command.Args,
	}).Info("Processing command")

	switch command.Command {
	case "/start":
		h.handleStart(ctx, command)
	case "/search":
		h.handleSearch(ctx, command)
	case "/profile":
		h.handleProfile(ctx, command)
	case "/help":
		h.handleHelp(ctx, command)
	default:
		h.sendMessage(ctx, command.ChatID, "Unknown command. Use /help to see available commands")
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

func (h *Handler) handleStart(ctx context.Context, cmd BotCommand) {
	welcomeMessage := "Welcome to My Media Search Bot!"

	h.logger.WithFields(logrus.Fields{
		"user_id": cmd.UserID,
		"chat_id": cmd.ChatID,
	}).Info("Sending start message")

	h.sendMessage(ctx, cmd.ChatID, welcomeMessage)
}

func (h *Handler) handleProfile(ctx context.Context, cmd BotCommand) {
	user, err := h.userService.GetUser(cmd.UserID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user profile")
		h.sendMessage(ctx, cmd.ChatID, "Sorry, I couldn't retrieve your profile information.")
		return
	}

	profileMessage := "<b>Your Profile:</b>\n\n"
	profileMessage += "User ID: " + user.ID + "\n"

	if user.Username != "" {
		profileMessage += "Username: @" + user.Username + "\n"
	}

	profileMessage += "Platform: " + user.Platform + "\n"
	profileMessage += "Member since: " + user.CreatedAt.Format("January 2, 2006") + "\n"

	if !user.UpdatedAt.Equal(user.CreatedAt) {
		profileMessage += "Last updated: " + user.UpdatedAt.Format("January 2, 2006") + "\n"
	}

	h.sendMessage(ctx, cmd.ChatID, profileMessage)
}

func (h *Handler) handleSearch(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) == 0 {
		h.sendMessage(ctx, cmd.ChatID, "Please provide an anime name to search. Example: /search Naruto")
		return
	}

	query := strings.Join(cmd.Args, " ")

	h.sendMessage(ctx, cmd.ChatID, "Searching for anime...")

	searchResult, err := h.animeService.SearchAnime(query)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search anime")
		h.sendMessage(ctx, cmd.ChatID, "Error occurred while searching. Please try again later.")
		return
	}

	message := services.FormatAnimeMessage(searchResult.Data)
	h.sendMessage(ctx, cmd.ChatID, message)
}

func (h *Handler) handleHelp(ctx context.Context, cmd BotCommand) {
	helpMessage := "Available Commands:\n\n/start - Show welcome message\n/search &lt;anime_name&gt; - Search for anime\n/profile - View your profile\n/help - Show this help"

	h.sendMessage(ctx, cmd.ChatID, helpMessage)
}

func (h *Handler) sendMessage(ctx context.Context, chatID, text string) {
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		h.logger.WithError(err).Error("Invalid chat ID")
		return
	}

	if err := services.SendTelegramMessage(ctx, h.botToken, chatIDInt, text); err != nil {
		h.logger.WithFields(logrus.Fields{
			"chat_id": chatIDInt,
			"error":   err.Error(),
		}).Error("Failed to send message")
	} else {
		h.logger.WithFields(logrus.Fields{
			"chat_id": chatIDInt,
		}).Debug("Message sent successfully")
	}
}
