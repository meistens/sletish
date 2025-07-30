package bot

import (
	"context"
	"fmt"
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
	userService  *services.UserService
	logger       *logrus.Logger
	botToken     string
	// UPDATE WITH MORE SERVICES ADDED IN THE FUTURE
}

func NewHandler(animeService *services.Client, userService *services.UserService, logger *logrus.Logger, botToken string) *Handler {
	return &Handler{
		animeService: animeService,
		userService:  userService,
		logger:       logger,
		botToken:     botToken,
	}
}

func (h *Handler) ProcessMessage(ctx context.Context, update *models.Update) {
	if update.Message.Text == "" {
		return
	}

	username := update.Message.From.Username
	userID := strconv.Itoa(update.Message.From.Id)
	chatID := strconv.Itoa(update.Message.Chat.Id)

	// Ensure user exists with proper error handling
	if err := h.userService.EnsureUserExists(userID, username); err != nil {
		h.logger.WithError(err).Error("failed to ensure user exists")
		h.sendMessage(ctx, chatID, "Sorry, I'm having trouble accessing your account. Please try again.")
		return
	}

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
	case "/add":
		h.handleAdd(ctx, command)
	case "/remove":
		h.handleRemove(ctx, command)
	case "/list":
		h.handleList(ctx, command)
	case "/update":
		h.handleUpdate(ctx, command)
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
	welcomeMessage := `<b>Welcome to Anime Tracker Bot!</b>

I can help you search for anime and manage your personal anime list.

<b>Available Commands:</b>
• /search &lt;anime_name&gt; - Search for anime
• /add &lt;anime_id&gt; &lt;status&gt; - Add anime to your list
• /list [status] - View your anime list
• /update &lt;anime_id&gt; &lt;new_status&gt; - Update anime status
• /remove &lt;anime_id&gt; - Remove anime from list
• /profile - View your profile
• /help - Show this help

<b>Valid statuses:</b> watching, completed, on_hold, dropped, watchlist

Get started by searching for an anime with /search!`

	h.logger.WithFields(logrus.Fields{
		"user_id": cmd.UserID,
		"chat_id": cmd.ChatID,
	}).Info("Sending start message")

	h.sendMessage(ctx, cmd.ChatID, welcomeMessage)
}

func (h *Handler) handleProfile(ctx context.Context, cmd BotCommand) {
	user, err := h.userService.GetUser(cmd.UserID)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"user_id": cmd.UserID,
			"error":   err.Error(),
		}).Error("Failed to get user profile")

		h.sendMessage(ctx, cmd.ChatID, "Sorry, I couldn't retrieve your profile information.")
		return
	}

	profileMessage := "<b>📋 Your Profile:</b>\n\n"
	profileMessage += "🆔 User ID: " + user.ID + "\n"

	if user.Username != "" {
		profileMessage += "👤 Username: @" + user.Username + "\n"
	}

	profileMessage += "📱 Platform: " + user.Platform + "\n"
	profileMessage += "📅 Member since: " + user.CreatedAt.Format("January 2, 2006") + "\n"

	if !user.UpdatedAt.Equal(user.CreatedAt) {
		profileMessage += "🔄 Last updated: " + user.UpdatedAt.Format("January 2, 2006") + "\n"
	}

	// Get user's anime stats
	allList, err := h.userService.GetUserList(cmd.UserID, "")
	if err == nil {
		statusCounts := make(map[models.Status]int)
		for _, item := range allList {
			statusCounts[item.UserMedia.Status]++
		}

		if len(statusCounts) > 0 {
			profileMessage += "\n<b>📊 Your Stats:</b>\n"
			if count := statusCounts[models.StatusWatching]; count > 0 {
				profileMessage += fmt.Sprintf("👀 Watching: %d\n", count)
			}
			if count := statusCounts[models.StatusCompleted]; count > 0 {
				profileMessage += fmt.Sprintf("✅ Completed: %d\n", count)
			}
			if count := statusCounts[models.StatusWatchlist]; count > 0 {
				profileMessage += fmt.Sprintf("📝 Watchlist: %d\n", count)
			}
			if count := statusCounts[models.StatusOnHold]; count > 0 {
				profileMessage += fmt.Sprintf("⏸ On Hold: %d\n", count)
			}
			if count := statusCounts[models.StatusDropped]; count > 0 {
				profileMessage += fmt.Sprintf("❌ Dropped: %d\n", count)
			}
		}
	}

	h.sendMessage(ctx, cmd.ChatID, profileMessage)
}

func (h *Handler) handleSearch(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) == 0 {
		h.sendMessage(ctx, cmd.ChatID, "Please provide an anime name to search. Example: /search Naruto")
		return
	}

	query := strings.Join(cmd.Args, " ")

	// Input validation
	if len(query) > 100 {
		h.sendMessage(ctx, cmd.ChatID, "Search query is too long. Please keep it under 100 characters.")
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "🔎 Searching for anime...")

	searchResult, err := h.animeService.SearchAnime(query)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"query":   query,
			"user_id": cmd.UserID,
			"error":   err.Error(),
		}).Error("Failed to search anime")

		h.sendMessage(ctx, cmd.ChatID, "❌ Error occurred while searching. Please try again later.")
		return
	}

	// no results found for query
	if len(searchResult.Data) == 0 {
		h.sendMessage(ctx, cmd.ChatID, "❌ No anime found matching your search")
		return
	}

	// TODO: refine formatting
	message := services.FormatAnimeMessage(searchResult.Data)
	message += "\n💡 <i>To add an anime to your list, use: /add &lt;anime_id&gt; &lt;status&gt;</i>"
	h.sendMessage(ctx, cmd.ChatID, message)
}

func (h *Handler) handleAdd(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) < 2 {
		h.sendMessage(ctx, cmd.ChatID, `<b>Usage:</b> /add &lt;anime_id&gt; &lt;status&gt;

<b>Valid statuses:</b>
• watching - Currently watching
• completed - Finished watching
• on_hold - Paused/on hold
• dropped - Stopped watching
• watchlist - Want to watch later

<b>Example:</b> /add 5114 watching`)
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "⏳ Adding anime to your list...")

	animeID, err := strconv.Atoi(cmd.Args[0])
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"cmd_args": cmd.Args,
			"user_id":  cmd.UserID,
			"error":    err.Error(),
		}).Warn("Invalid anime ID")

		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid anime ID. Please use a valid numeric ID from search results.")
		return
	}

	status := models.Status(cmd.Args[1])
	if !isValidStatus(status) {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid status. Valid options are: watching, completed, on_hold, dropped, watchlist")
		return
	}

	// add to user personalized list
	if err := h.userService.AddToUserList(cmd.UserID, animeID, status); err != nil {
		h.logger.WithError(err).Error("Failed to add anime to user list")

		if strings.Contains(err.Error(), "not found") {
			h.sendMessage(ctx, cmd.ChatID, "❌ Anime with that ID doesn't exist. Please check the ID from search results.")
		} else {
			h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't add the anime to your list. Please try again later.")
		}
		return
	}

	h.sendMessage(ctx, cmd.ChatID, fmt.Sprintf("✅ Successfully added anime to your list with status: <b>%s</b>", status))
}

func (h *Handler) handleRemove(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) < 1 {
		h.sendMessage(ctx, cmd.ChatID, `<b>Usage:</b> /remove &lt;anime_id&gt;

<b>Example:</b> /remove 5114`)
		return
	}

	animeID, err := strconv.Atoi(cmd.Args[0])
	if err != nil {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid anime ID. Please use a valid numeric ID.")
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "⏳ Removing anime from your list...")

	if err := h.userService.RemoveFromUserList(cmd.UserID, animeID); err != nil {
		h.logger.WithError(err).Error("Failed to remove anime from user list")

		if strings.Contains(err.Error(), "not found") {
			h.sendMessage(ctx, cmd.ChatID, "❌ Anime not found in your list.")
		} else {
			h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't remove the anime from your list. Please try again later.")
		}
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "✅ Successfully removed anime from your list.")
}

func (h *Handler) handleList(ctx context.Context, cmd BotCommand) {
	var status models.Status
	if len(cmd.Args) > 0 {
		status = models.Status(cmd.Args[0])
		if status != "" && !isValidStatus(status) {
			h.sendMessage(ctx, cmd.ChatID, "❌ Invalid status. Valid options are: watching, completed, on_hold, dropped, watchlist")
			return
		}
	}

	h.sendMessage(ctx, cmd.ChatID, "📋 Getting your anime list...")

	userList, err := h.userService.GetUserList(cmd.UserID, status)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user list")
		h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't retrieve your anime list. Please try again later.")
		return
	}

	if len(userList) == 0 {
		if status != "" {
			h.sendMessage(ctx, cmd.ChatID, fmt.Sprintf("📭 You don't have any anime with status '%s' in your list.", status))
		} else {
			h.sendMessage(ctx, cmd.ChatID, "📭 Your anime list is empty. Use /search to find anime and /add to add them!")
		}
		return
	}

	message := h.formatUserList(userList, status)
	h.sendMessage(ctx, cmd.ChatID, message)
}

func (h *Handler) handleUpdate(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) < 2 {
		h.sendMessage(ctx, cmd.ChatID, `<b>Usage:</b> /update &lt;anime_id&gt; &lt;new_status&gt;

<b>Valid statuses:</b>
• watching, completed, on_hold, dropped, watchlist

<b>Example:</b> /update 5114 completed`)
		return
	}

	animeID, err := strconv.Atoi(cmd.Args[0])
	if err != nil {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid anime ID. Please use a valid numeric ID.")
		return
	}

	status := models.Status(cmd.Args[1])
	if !isValidStatus(status) {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid status. Valid options are: watching, completed, on_hold, dropped, watchlist")
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "⏳ Updating anime status...")

	if err := h.userService.UpdateAnimeStatus(cmd.UserID, animeID, status); err != nil {
		h.logger.WithError(err).Error("Failed to update anime status")

		if strings.Contains(err.Error(), "not found") {
			h.sendMessage(ctx, cmd.ChatID, "❌ Anime not found in your list. Use /add to add it first.")
		} else {
			h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't update the anime status. Please try again later.")
		}
		return
	}

	h.sendMessage(ctx, cmd.ChatID, fmt.Sprintf("✅ Successfully updated anime status to: <b>%s</b>", status))
}

func (h *Handler) handleHelp(ctx context.Context, cmd BotCommand) {
	helpMessage := `<b>🤖 Anime Tracker Bot - Help</b>

<b>📝 Commands:</b>

<b>/start</b> - Show welcome message
<b>/search</b> &lt;anime_name&gt; - Search for anime
<b>/add</b> &lt;anime_id&gt; &lt;status&gt; - Add anime to your list
<b>/list</b> [status] - View your anime list (all or by status)
<b>/update</b> &lt;anime_id&gt; &lt;new_status&gt; - Update anime status
<b>/remove</b> &lt;anime_id&gt; - Remove anime from your list
<b>/profile</b> - View your profile and stats
<b>/help</b> - Show this help message

<b>📊 Valid Statuses:</b>
• <code>watching</code> - Currently watching
• <code>completed</code> - Finished watching
• <code>on_hold</code> - Paused/on hold
• <code>dropped</code> - Stopped watching
• <code>watchlist</code> - Want to watch later

<b>💡 Examples:</b>
<code>/search Attack on Titan</code>
<code>/add 16498 watching</code>
<code>/list completed</code>
<code>/update 16498 completed</code>

Need more help? Just ask!`

	h.sendMessage(ctx, cmd.ChatID, helpMessage)
}

func (h *Handler) formatUserList(userList []models.UserMediaWithDetails, filterStatus models.Status) string {
	var message strings.Builder

	if filterStatus != "" {
		message.WriteString(fmt.Sprintf("<b>📋 Your %s Anime List:</b>\n\n", strings.Title(string(filterStatus))))
	} else {
		message.WriteString("<b>📋 Your Anime List:</b>\n\n")
	}

	// Group by status if showing all
	if filterStatus == "" {
		statusGroups := make(map[models.Status][]models.UserMediaWithDetails)
		for _, item := range userList {
			statusGroups[item.UserMedia.Status] = append(statusGroups[item.UserMedia.Status], item)
		}

		// Order statuses logically
		orderedStatuses := []models.Status{
			models.StatusWatching,
			models.StatusCompleted,
			models.StatusWatchlist,
			models.StatusOnHold,
			models.StatusDropped,
		}

		for _, status := range orderedStatuses {
			items := statusGroups[status]
			if len(items) == 0 {
				continue
			}

			statusEmoji := getStatusEmoji(status)
			message.WriteString(fmt.Sprintf("<b>%s %s (%d):</b>\n", statusEmoji, strings.Title(string(status)), len(items)))

			for i, item := range items {
				if i >= 5 { // Limit to 5 per status to avoid long messages
					message.WriteString(fmt.Sprintf("   ... and %d more\n", len(items)-5))
					break
				}
				message.WriteString(fmt.Sprintf("   • %s (ID: %s)\n",
					item.Media.Title, item.Media.ExternalID))
			}
			message.WriteString("\n")
		}
	} else {
		// Show detailed list for specific status
		statusEmoji := getStatusEmoji(filterStatus)
		for i, item := range userList {
			if i >= 20 { // Limit to 20 items
				message.WriteString(fmt.Sprintf("\n... and %d more items\n", len(userList)-20))
				break
			}

			message.WriteString(fmt.Sprintf("%s <b>%s</b>\n", statusEmoji, item.Media.Title))
			message.WriteString(fmt.Sprintf("   🆔 ID: %s", item.Media.ExternalID))

			if item.Media.Rating > 0 {
				message.WriteString(fmt.Sprintf(" | ⭐ %.1f", item.Media.Rating))
			}

			if item.Media.ReleaseDate != "" {
				message.WriteString(fmt.Sprintf(" | 📅 %s", item.Media.ReleaseDate))
			}

			message.WriteString(fmt.Sprintf("\n   📝 Added: %s\n\n",
				item.UserMedia.CreatedAt.Format("Jan 2, 2006")))
		}
	}

	message.WriteString("<i>Use /update &lt;id&gt; &lt;status&gt; to change status or /remove &lt;id&gt; to remove</i>")
	return message.String()
}

func getStatusEmoji(status models.Status) string {
	switch status {
	case models.StatusWatching:
		return "👀"
	case models.StatusCompleted:
		return "✅"
	case models.StatusWatchlist:
		return "📝"
	case models.StatusOnHold:
		return "⏸"
	case models.StatusDropped:
		return "❌"
	default:
		return "📺"
	}
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

func isValidStatus(status models.Status) bool {
	validStatuses := []models.Status{
		models.StatusCompleted,
		models.StatusDropped,
		models.StatusOnHold,
		models.StatusWatching,
		models.StatusWatchlist,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
