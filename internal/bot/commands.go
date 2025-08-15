package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"sletish/internal/models"
	"sletish/internal/services"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type BotCommand struct {
	Command string
	Args    []string
	UserID  string
	ChatID  string
}

type Handler struct {
	animeService    *services.Client
	userService     *services.UserService
	reminderService *services.ReminderService
	logger          *logrus.Logger
	botToken        string
	// UPDATE WITH MORE SERVICES ADDED IN THE FUTURE
}

func NewHandler(animeService *services.Client, userService *services.UserService, reminderService *services.ReminderService, logger *logrus.Logger, botToken string) *Handler {
	return &Handler{
		animeService:    animeService,
		userService:     userService,
		reminderService: reminderService,
		logger:          logger,
		botToken:        botToken,
	}
}

func (h *Handler) ProcessMessage(ctx context.Context, update *models.Update) {
	// Handle callback queries (button clicks)
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}

	// Handle regular messages
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
	case "/remind":
		h.handleRemind(ctx, command)
	case "/reminders":
		h.handleReminders(ctx, command)
	default:
		h.sendMessage(ctx, command.ChatID, "Unknown command. Use /help to see available commands")
	}
}

func (h *Handler) handleRemind(ctx context.Context, cmd BotCommand) {
	if len(cmd.Args) < 3 {
		h.sendMessage(ctx, cmd.ChatID, `<b>Usage:</b> /remind &lt;anime_id&gt; &lt;days&gt; &lt;message&gt;

			<b>Examples:</b>
			• /remind 5114 7 "Check if new episode is out!"
			• /remind 16498 30 "Time to rewatch this masterpiece"

			<b>Note:</b> Days IS 1-365`)
		return
	}

	animeID, err := strconv.Atoi(cmd.Args[0])
	if err != nil {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid anime ID. Please use a valid numeric ID from search results.")
		return
	}

	days, err := strconv.Atoi(cmd.Args[1])
	if err != nil || days < 1 || days > 365 {
		h.sendMessage(ctx, cmd.ChatID, "❌ Invalid number of days. Please use 1-365 days.")
		return
	}

	message := strings.Join(cmd.Args[2:], " ")
	if len(message) > 200 {
		h.sendMessage(ctx, cmd.ChatID, "❌ Message too long. Please keep it under 200 characters.")
		return
	}

	h.sendMessage(ctx, cmd.ChatID, "⏳ Setting up your reminder...")

	remindAt := time.Now().AddDate(0, 0, days)

	if err := h.reminderService.CreateReminder(cmd.UserID, animeID, message, remindAt); err != nil {
		h.logger.WithError(err).Error("Failed to create reminder")

		if strings.Contains(err.Error(), "does not exist") {
			h.sendMessage(ctx, cmd.ChatID, "❌ Anime with that ID doesn't exist. Please check the ID from search results.")
		} else {
			h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't create the reminder. Please try again later.")
		}

		return
	}

	h.sendMessage(ctx, cmd.ChatID, fmt.Sprintf("✅ Reminder set! I'll remind you on <b>%s</b> with message: \"%s\"",
		remindAt.Format("January 2, 2006 at 3:04 PM"), message))
}

func (h *Handler) handleReminders(ctx context.Context, cmd BotCommand) {
	showAll := false
	if len(cmd.Args) > 0 && strings.ToLower(cmd.Args[0]) == "all" {
		showAll = true
	}

	reminders, err := h.reminderService.GetUserReminders(cmd.UserID, showAll)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user reminders")
		h.sendMessage(ctx, cmd.ChatID, "❌ Sorry, I couldn't retrieve your reminders. Please try again later.")
		return
	}

	if len(reminders) == 0 {
		if showAll {
			h.sendMessage(ctx, cmd.ChatID, "📝 You have no reminders.\n\nUse /remind to set up reminders for your anime!")
		} else {
			h.sendMessage(ctx, cmd.ChatID, "📝 You have no pending reminders.\n\nUse /reminders all to see all reminders, or /remind to set up new ones!")
		}
		return
	}

	message := h.formatReminders(reminders, showAll)
	keyboard := h.createRemindersKeyboard(reminders)
	h.sendMessageWithKeyboard(ctx, cmd.ChatID, message, keyboard)
}

func (h *Handler) formatReminders(reminders []models.Reminder, showAll bool) string {
	var message strings.Builder

	if showAll {
		message.WriteString("<b>📝 All Your Reminders</b>\n\n")
	} else {
		message.WriteString("<b>📝 Your Pending Reminders</b>\n\n")
	}

	now := time.Now()
	pending := 0
	sent := 0

	for i, reminder := range reminders {
		if i >= 10 { // Limit display to 10 reminders
			message.WriteString(fmt.Sprintf("... and %d more reminders\n", len(reminders)-10))
			break
		}

		status := "📅"
		statusText := "Pending"
		timeText := reminder.RemindAt.Format("Jan 2, 2006 3:04 PM")

		if reminder.Sent {
			status = "✅"
			statusText = "Sent"
			sent++
		} else {
			pending++
			if reminder.RemindAt.Before(now) {
				status = "🔔"
				statusText = "Due"
			}
		}

		message.WriteString(fmt.Sprintf("%s <b>%s</b> - %s\n", status, statusText, timeText))

		if reminder.MediaTitle != "" {
			message.WriteString(fmt.Sprintf("   🎬 <i>%s</i>\n", reminder.MediaTitle))
		} else {
			message.WriteString(fmt.Sprintf("   🆔 Anime ID: %d\n", reminder.MediaID))
		}

		message.WriteString(fmt.Sprintf("   💬 \"%s\"\n", reminder.Message))
		message.WriteString(fmt.Sprintf("   📅 Created: %s\n\n", reminder.CreatedAt.Format("Jan 2, 2006")))
	}

	// Summary
	message.WriteString(fmt.Sprintf("<b>📊 Summary:</b>\n"))
	if !showAll {
		message.WriteString(fmt.Sprintf("📅 Pending: %d\n", pending))
		message.WriteString(fmt.Sprintf("\n💡 <i>Use /reminders all to see all reminders</i>"))
	} else {
		message.WriteString(fmt.Sprintf("📅 Pending: %d | ✅ Sent: %d\n", pending, sent))
	}

	return message.String()
}

func (h *Handler) createRemindersKeyboard(reminders []models.Reminder) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	// Show first few pending reminders with cancel option
	pendingCount := 0
	for _, reminder := range reminders {
		if !reminder.Sent && pendingCount < 3 {
			title := reminder.MediaTitle
			if title == "" {
				title = fmt.Sprintf("Anime ID: %d", reminder.MediaID)
			}
			if len(title) > 25 {
				title = title[:25] + "..."
			}

			// Create callback data manually since we need reminder ID, not anime ID
			callbackData := models.CallbackData{
				Action:  "cancel_reminder",
				AnimeID: strconv.Itoa(reminder.ID),
			}
			jsonData, _ := json.Marshal(callbackData)

			cancelRow := []models.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("🗑 Cancel: %s", title),
					CallbackData: string(jsonData),
				},
			}
			rows = append(rows, cancelRow)
			pendingCount++
		}
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func (h *Handler) handleCallbackQuery(ctx context.Context, callback *models.CallbackQuery) {
	h.logger.WithFields(logrus.Fields{
		"callback_id": callback.Id,
		"user_id":     callback.From.Id,
		"data":        callback.Data,
	}).Info("Processing callback query")

	var callbackData models.CallbackData
	if err := json.Unmarshal([]byte(callback.Data), &callbackData); err != nil {
		h.logger.WithError(err).Error("Failed to parse callback data")
		h.answerCallback(ctx, callback.Id, "❌ Error processing request", false)
		return
	}

	userID := strconv.Itoa(callback.From.Id)
	chatID := strconv.Itoa(callback.Message.Chat.Id)

	switch callbackData.Action {
	case "add_anime":
		h.handleCallbackAddAnime(ctx, callback, &callbackData, userID, chatID)
	case "update_status":
		h.handleCallbackUpdateStatus(ctx, callback, &callbackData, userID, chatID)
	case "remove_anime":
		h.handleCallbackRemoveAnime(ctx, callback, &callbackData, userID, chatID)
	case "view_details":
		h.handleCallbackViewDetails(ctx, callback, &callbackData, userID, chatID)
	case "list_page":
		h.handleCallbackListPage(ctx, callback, &callbackData, userID, chatID)
	case "cancel_reminder":
		h.handleCallbackCancelReminder(ctx, callback, &callbackData, userID, chatID)

	default:
		h.answerCallback(ctx, callback.Id, "❌ Unknown action", false)
	}
}

func (h *Handler) handleCallbackCancelReminder(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	if data.AnimeID == "" { // Using AnimeID field to store reminder ID
		h.answerCallback(ctx, callback.Id, "❌ Invalid reminder ID", false)
		return
	}

	reminderID, err := strconv.Atoi(data.AnimeID)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Invalid reminder ID", false)
		return
	}

	if err := h.reminderService.CancelReminder(userID, reminderID); err != nil {
		h.logger.WithError(err).Error("Failed to cancel reminder")
		if strings.Contains(err.Error(), "not found") {
			h.answerCallback(ctx, callback.Id, "❌ Reminder not found", true)
		} else {
			h.answerCallback(ctx, callback.Id, "❌ Failed to cancel reminder", true)
		}
		return
	}

	h.answerCallback(ctx, callback.Id, "✅ Reminder cancelled!", false)

	// Update the message
	newText := "✅ <b>Reminder cancelled successfully!</b>\n\nUse /reminders to view your remaining reminders."
	h.editMessage(ctx, chatID, callback.Message.MessageId, newText, nil)
}

func (h *Handler) handleCallbackAddAnime(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	if data.AnimeID == "" || data.Status == "" {
		h.answerCallback(ctx, callback.Id, "❌ Invalid data", false)
		return
	}

	animeID, err := strconv.Atoi(data.AnimeID)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	status := models.Status(data.Status)
	if !isValidStatus(status) {
		h.answerCallback(ctx, callback.Id, "❌ Invalid status", false)
		return
	}

	if err := h.userService.AddToUserList(userID, animeID, status); err != nil {
		h.logger.WithError(err).Error("Failed to add anime via callback")
		if strings.Contains(err.Error(), "not found") {
			h.answerCallback(ctx, callback.Id, "❌ Anime not found", true)
		} else {
			h.answerCallback(ctx, callback.Id, "❌ Failed to add anime", true)
		}
		return
	}

	h.answerCallback(ctx, callback.Id, fmt.Sprintf("✅ Added to %s list!", status), false)

	// Update the message to show it was added
	newText := fmt.Sprintf("✅ <b>Anime added to your %s list!</b>\n\nUse /list to view your anime list.", status)
	h.editMessage(ctx, chatID, callback.Message.MessageId, newText, nil)
}

func (h *Handler) handleCallbackUpdateStatus(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	if data.AnimeID == "" || data.Status == "" {
		h.answerCallback(ctx, callback.Id, "❌ Invalid data", false)
		return
	}

	animeID, err := strconv.Atoi(data.AnimeID)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	status := models.Status(data.Status)
	if !isValidStatus(status) {
		h.answerCallback(ctx, callback.Id, "❌ Invalid status", false)
		return
	}

	if err := h.userService.UpdateAnimeStatus(userID, animeID, status); err != nil {
		h.logger.WithError(err).Error("Failed to update anime status via callback")
		if strings.Contains(err.Error(), "not found") {
			h.answerCallback(ctx, callback.Id, "❌ Anime not found in your list", true)
		} else {
			h.answerCallback(ctx, callback.Id, "❌ Failed to update status", true)
		}
		return
	}

	h.answerCallback(ctx, callback.Id, fmt.Sprintf("✅ Status updated to %s!", status), false)
}

func (h *Handler) handleCallbackRemoveAnime(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	if data.AnimeID == "" {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	animeID, err := strconv.Atoi(data.AnimeID)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	if err := h.userService.RemoveFromUserList(userID, animeID); err != nil {
		h.logger.WithError(err).Error("Failed to remove anime via callback")
		if strings.Contains(err.Error(), "not found") {
			h.answerCallback(ctx, callback.Id, "❌ Anime not found in your list", true)
		} else {
			h.answerCallback(ctx, callback.Id, "❌ Failed to remove anime", true)
		}
		return
	}

	h.answerCallback(ctx, callback.Id, "✅ Anime removed from your list!", false)
}

func (h *Handler) handleCallbackViewDetails(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	if data.AnimeID == "" {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	animeID, err := strconv.Atoi(data.AnimeID)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Invalid anime ID", false)
		return
	}

	anime, err := h.animeService.GetAnimeByID(animeID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get anime details via callback")
		h.answerCallback(ctx, callback.Id, "❌ Failed to get anime details", true)
		return
	}

	detailsMessage := h.formatAnimeDetails(*anime)
	keyboard := h.createAnimeDetailsKeyboard(data.AnimeID)

	h.editMessage(ctx, chatID, callback.Message.MessageId, detailsMessage, keyboard)
	h.answerCallback(ctx, callback.Id, "", false)
}

// handleCallbackListPage processes pagination button clicks for the user's list.
func (h *Handler) handleCallbackListPage(ctx context.Context, callback *models.CallbackQuery, data *models.CallbackData, userID, chatID string) {
	userList, total, err := h.userService.GetUserList(userID, data.Status, data.Page, data.Limit)
	if err != nil {
		h.answerCallback(ctx, callback.Id, "❌ Failed to get list.", true)
		return
	}

	if len(userList) == 0 {
		h.answerCallback(ctx, callback.Id, "Your list is empty!", true)
		return
	}

	message := h.formatUserList(userList, data.Status, data.Page, total, data.Limit)
	keyboard := h.createPaginationKeyboard(data.Page, data.Limit, total, data.Status)

	h.editMessage(ctx, chatID, callback.Message.MessageId, message, keyboard)
	h.answerCallback(ctx, callback.Id, "", false)
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

	if user.Username != nil && *user.Username != "" {
		profileMessage += "👤 Username: @" + *user.Username + "\n"
	}

	profileMessage += "📱 Platform: " + user.Platform + "\n"
	profileMessage += "📅 Member since: " + user.CreatedAt.Format("January 2, 2006") + "\n"

	if !user.UpdatedAt.Equal(user.CreatedAt) {
		profileMessage += "🔄 Last updated: " + user.UpdatedAt.Format("January 2, 2006") + "\n"
	}

	// Get user's anime stats
	allList, _, err := h.userService.GetUserList(cmd.UserID, "", 1, 1000) // Get all items for stats (later implementation)
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

	// Format message with interactive keyboards
	message := h.formatSearchResults(searchResult.Data)
	keyboard := h.createSearchResultsKeyboard(searchResult.Data)

	h.sendMessageWithKeyboard(ctx, cmd.ChatID, message, keyboard)
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

// handleList fetches and displays the user's anime list with pagination.
func (h *Handler) handleList(ctx context.Context, cmd BotCommand) {
	var statusFilter string
	page := 1
	limit := 5 // Default limit per page, no more, maybe less

	// Parse arguments: /list [status] [page]
	if len(cmd.Args) > 0 {
		firstArg := strings.ToLower(cmd.Args[0])
		if isValidStatus(models.Status(firstArg)) {
			statusFilter = firstArg
			// Check if there's a page number after the status
			if len(cmd.Args) > 1 {
				if p, err := strconv.Atoi(cmd.Args[1]); err == nil && p > 0 {
					page = p
				}
			}
		} else {
			// First argument is not a valid status, check if it's a page number
			if p, err := strconv.Atoi(firstArg); err == nil && p > 0 {
				page = p
			}
		}
	}

	userList, total, err := h.userService.GetUserList(cmd.UserID, statusFilter, page, limit)
	if err != nil {
		h.sendMessage(ctx, cmd.ChatID, "Failed to get your list: "+err.Error())
		return
	}

	if len(userList) == 0 {
		if statusFilter != "" {
			h.sendMessage(ctx, cmd.ChatID, fmt.Sprintf("Your %s list is empty!", statusFilter))
		} else {
			h.sendMessage(ctx, cmd.ChatID, "Your anime list is empty! Use /search to find anime and add them to your list.")
		}
		return
	}

	message := h.formatUserList(userList, statusFilter, page, total, limit)
	keyboard := h.createPaginationKeyboard(page, limit, total, statusFilter)
	h.sendMessageWithKeyboard(ctx, cmd.ChatID, message, keyboard)
}

// createPaginationKeyboard generates an inline keyboard with pagination buttons.
func (h *Handler) createPaginationKeyboard(currentPage, limit, total int, statusFilter string) *models.InlineKeyboardMarkup {
	var buttons []models.InlineKeyboardButton

	// Previous page button
	if currentPage > 1 {
		callbackData := models.CallbackData{
			Action: "list_page",
			Page:   currentPage - 1,
			Limit:  limit,
			Total:  total,
			Status: statusFilter,
		}
		data, _ := json.Marshal(callbackData)
		buttons = append(buttons, models.InlineKeyboardButton{Text: "⬅️ Previous", CallbackData: string(data)})
	}

	// Current page info
	totalPages := (total + limit - 1) / limit
	pageInfo := fmt.Sprintf("📄 %d/%d", currentPage, totalPages)
	buttons = append(buttons, models.InlineKeyboardButton{Text: pageInfo, CallbackData: "noop"})

	// Next page button
	if currentPage*limit < total {
		callbackData := models.CallbackData{
			Action: "list_page",
			Page:   currentPage + 1,
			Limit:  limit,
			Total:  total,
			Status: statusFilter,
		}
		data, _ := json.Marshal(callbackData)
		buttons = append(buttons, models.InlineKeyboardButton{Text: "Next ➡️", CallbackData: string(data)})
	}

	if len(buttons) <= 1 { // Only page info button
		return nil
	}

	keyboard := models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{buttons},
	}
	return &keyboard
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
<b>/list</b> [status] [page] - View your anime list (all or by status)
<b>/update</b> &lt;anime_id&gt; &lt;new_status&gt; - Update anime status
<b>/remove</b> &lt;anime_id&gt; - Remove anime from your list
<b>/profile</b> - View your profile and stats
<b>/remind</b> &lt;anime_id&gt; &lt;days&gt; &lt;message&gt; - Set reminder
<b>/reminders</b> [all] - View your reminders
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
<code>/list watching 2</code>
<code>/update 16498 completed</code>
<code>/remind 16498 30 "Time to rewatch!"</code>
<code>/reminders</code>

Need more help? Just ask!`

	h.sendMessage(ctx, cmd.ChatID, helpMessage)
}

// Keyboard creation methods
func (h *Handler) createSearchResultsKeyboard(animes []models.AnimeData) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	// Add quick action buttons for first result
	if len(animes) > 0 {
		firstAnime := animes[0]
		animeID := strconv.Itoa(firstAnime.MalID)

		// Status selection row
		statusRow := []models.InlineKeyboardButton{
			{
				Text:         "📝 Watchlist",
				CallbackData: h.createCallbackData("add_anime", animeID, "watchlist"),
			},
			{
				Text:         "👀 Watching",
				CallbackData: h.createCallbackData("add_anime", animeID, "watching"),
			},
		}
		rows = append(rows, statusRow)

		// More status options
		statusRow2 := []models.InlineKeyboardButton{
			{
				Text:         "✅ Completed",
				CallbackData: h.createCallbackData("add_anime", animeID, "completed"),
			},
			{
				Text:         "⏸ On Hold",
				CallbackData: h.createCallbackData("add_anime", animeID, "on_hold"),
			},
		}
		rows = append(rows, statusRow2)

		// Details and external link row
		detailsRow := []models.InlineKeyboardButton{
			{
				Text:         "📖 Details",
				CallbackData: h.createCallbackData("view_details", animeID, ""),
			},
			{
				Text: "🔗 MyAnimeList",
				URL:  fmt.Sprintf("https://myanimelist.net/anime/%d", firstAnime.MalID),
			},
		}
		rows = append(rows, detailsRow)
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// Too much clutter, keep just-in-case
// func (h *Handler) createUserListKeyboard(userList []models.UserMediaWithDetails, filterStatus models.Status) *models.InlineKeyboardMarkup {
// 	var rows [][]models.InlineKeyboardButton

// 	// If showing a single status, add management buttons for first few items
// 	if filterStatus != "" && len(userList) > 0 {
// 		for i, item := range userList {
// 			if i >= 3 { // Limit to first 3 items to avoid too many buttons
// 				break
// 			}

// 			animeID := item.Media.ExternalID
// 			title := item.Media.Title
// 			if len(title) > 20 {
// 				title = title[:20] + "..."
// 			}

// 			// Status update buttons
// 			statusRow := []models.InlineKeyboardButton{
// 				{
// 					Text:         fmt.Sprintf("📝 %s", title),
// 					CallbackData: h.createCallbackData("view_details", animeID, ""),
// 				},
// 			}

// 			// Add status change button based on current status
// 			switch item.UserMedia.Status {
// 			case models.StatusWatching:
// 				statusRow = append(statusRow, models.InlineKeyboardButton{
// 					Text:         "✅ Complete",
// 					CallbackData: h.createCallbackData("update_status", animeID, "completed"),
// 				})
// 			case models.StatusWatchlist:
// 				statusRow = append(statusRow, models.InlineKeyboardButton{
// 					Text:         "👀 Start Watching",
// 					CallbackData: h.createCallbackData("update_status", animeID, "watching"),
// 				})
// 			case models.StatusCompleted:
// 				statusRow = append(statusRow, models.InlineKeyboardButton{
// 					Text:         "🗑 Remove",
// 					CallbackData: h.createCallbackData("remove_anime", animeID, ""),
// 				})
// 			}

// 			rows = append(rows, statusRow)
// 		}
// 	}

// 	// Filter buttons row
// 	if filterStatus == "" {
// 		filterRow := []models.InlineKeyboardButton{
// 			{
// 				Text:         "👀 Watching",
// 				CallbackData: h.createCallbackData("list_page", "", "watching"),
// 			},
// 			{
// 				Text:         "✅ Completed",
// 				CallbackData: h.createCallbackData("list_page", "", "completed"),
// 			},
// 		}
// 		rows = append(rows, filterRow)

// 		filterRow2 := []models.InlineKeyboardButton{
// 			{
// 				Text:         "📝 Watchlist",
// 				CallbackData: h.createCallbackData("list_page", "", "watchlist"),
// 			},
// 			{
// 				Text:         "⏸ On Hold",
// 				CallbackData: h.createCallbackData("list_page", "", "on_hold"),
// 			},
// 		}
// 		rows = append(rows, filterRow2)
// 	}

// 	return &models.InlineKeyboardMarkup{
// 		InlineKeyboard: rows,
// 	}
// }

func (h *Handler) createAnimeDetailsKeyboard(animeID string) *models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{
		{
			{
				Text:         "📝 Add to Watchlist",
				CallbackData: h.createCallbackData("add_anime", animeID, "watchlist"),
			},
			{
				Text:         "👀 Start Watching",
				CallbackData: h.createCallbackData("add_anime", animeID, "watching"),
			},
		},
		{
			{
				Text:         "✅ Mark Completed",
				CallbackData: h.createCallbackData("add_anime", animeID, "completed"),
			},
		},
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func (h *Handler) createCallbackData(action, animeID, status string) string {
	data := models.CallbackData{
		Action:  action,
		AnimeID: animeID,
		Status:  status,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal callback data")
		return "{}"
	}

	return string(jsonData)
}

// Enhanced formatting methods
func (h *Handler) formatSearchResults(animes []models.AnimeData) string {
	if len(animes) == 0 {
		return "No anime found for your search query."
	}

	var message strings.Builder
	message.WriteString("<b>🔍 Search Results</b>\n\n")

	// Show detailed info for first result
	anime := animes[0]
	message.WriteString(fmt.Sprintf("<b>%s</b>\n", anime.Title))
	message.WriteString(fmt.Sprintf("🆔 ID: <code>%d</code>", anime.MalID))

	if anime.Score > 0 {
		message.WriteString(fmt.Sprintf(" | ⭐ %.1f", anime.Score))
	}
	if anime.Episodes > 0 {
		message.WriteString(fmt.Sprintf(" | 📺 %d eps", anime.Episodes))
	}
	if anime.Year > 0 {
		message.WriteString(fmt.Sprintf(" | 📅 %d", anime.Year))
	}
	message.WriteString("\n")

	// Type and Status
	var details []string
	if anime.Type != "" {
		details = append(details, fmt.Sprintf("📱 %s", anime.Type))
	}
	if anime.Status != "" {
		details = append(details, fmt.Sprintf("📊 %s", anime.Status))
	}
	if len(details) > 0 {
		message.WriteString(strings.Join(details, " | ") + "\n")
	}

	// Synopsis (shortened)
	if anime.Synopsis != "" {
		synopsis := anime.Synopsis
		if len(synopsis) > 200 {
			synopsis = synopsis[:200] + "..."
		}
		message.WriteString(fmt.Sprintf("📝 %s\n", synopsis))
	}

	// Show other results briefly
	if len(animes) > 1 {
		message.WriteString(fmt.Sprintf("\n<b>Other Results (%d more):</b>\n", len(animes)-1))
		for i, otherAnime := range animes[1:] {
			if i >= 4 { // Show max 5 more
				message.WriteString(fmt.Sprintf("... and %d more results\n", len(animes)-6))
				break
			}
			message.WriteString(fmt.Sprintf("• %s (ID: %d)", otherAnime.Title, otherAnime.MalID))
			if otherAnime.Score > 0 {
				message.WriteString(fmt.Sprintf(" - ⭐ %.1f", otherAnime.Score))
			}
			message.WriteString("\n")
		}
	}

	message.WriteString("\n💡 <i>Use the buttons below to quickly add the top result to your list!</i>")
	return message.String()
}

func (h *Handler) formatAnimeDetails(anime models.AnimeData) string {
	var message strings.Builder
	message.WriteString(fmt.Sprintf("<b>📺 %s</b>\n\n", anime.Title))

	message.WriteString(fmt.Sprintf("🆔 ID: <code>%d</code>\n", anime.MalID))

	if anime.Score > 0 {
		message.WriteString(fmt.Sprintf("⭐ Rating: %.1f/10\n", anime.Score))
	}

	if anime.Episodes > 0 {
		message.WriteString(fmt.Sprintf("📺 Episodes: %d\n", anime.Episodes))
	}

	if anime.Year > 0 {
		message.WriteString(fmt.Sprintf("📅 Year: %d\n", anime.Year))
	}

	if anime.Type != "" {
		message.WriteString(fmt.Sprintf("📱 Type: %s\n", anime.Type))
	}

	if anime.Status != "" {
		message.WriteString(fmt.Sprintf("📊 Status: %s\n", anime.Status))
	}

	// Genres
	if len(anime.Genres) > 0 {
		genres := make([]string, 0, len(anime.Genres))
		for _, genre := range anime.Genres {
			genres = append(genres, genre.Name)
		}
		message.WriteString(fmt.Sprintf("🏷 Genres: %s\n", strings.Join(genres, ", ")))
	}

	// Synopsis
	if anime.Synopsis != "" {
		message.WriteString(fmt.Sprintf("\n📝 <b>Synopsis:</b>\n%s\n", anime.Synopsis))
	}

	message.WriteString(fmt.Sprintf("\n🔗 <a href=\"https://myanimelist.net/anime/%d\">View on MyAnimeList</a>", anime.MalID))

	return message.String()
}

// Helper functions to safely get float64 value from pointer
func getFloatValue(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// End

func (h *Handler) formatUserList(userList []models.UserMediaWithDetails, statusFilter string, page, total, limit int) string {
	var message strings.Builder

	// Calculate pagination info
	totalPages := (total + limit - 1) / limit
	start := (page-1)*limit + 1
	end := start + len(userList) - 1

	if statusFilter != "" {
		message.WriteString(fmt.Sprintf("<b>📋 Your %s Anime List</b>\n", strings.Title(statusFilter)))
	} else {
		message.WriteString("<b>📋 Your Anime List</b>\n")
	}

	message.WriteString(fmt.Sprintf("📄 Page %d of %d | Items %d-%d of %d\n\n", page, totalPages, start, end, total))

	// Group by status if showing all
	if statusFilter == "" {
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

			for _, item := range items {
				message.WriteString(fmt.Sprintf("   • %s (ID: %s)\n",
					item.Media.Title, item.Media.ExternalID))
			}
			message.WriteString("\n")
		}
	} else {
		// Show detailed list for specific status
		statusEmoji := getStatusEmoji(models.Status(statusFilter))
		for _, item := range userList {
			message.WriteString(fmt.Sprintf("%s <b>%s</b>\n", statusEmoji, item.Media.Title))
			message.WriteString(fmt.Sprintf("   🆔 ID: %s", item.Media.ExternalID))

			// Handle nullable rating for Media
			if item.Media.Rating != nil && *item.Media.Rating > 0 {
				message.WriteString(fmt.Sprintf(" | ⭐ %.1f", *item.Media.Rating))
			}

			// Handle nullable release date
			if item.Media.ReleaseDate != nil && *item.Media.ReleaseDate != "" {
				message.WriteString(fmt.Sprintf(" | 📅 %s", *item.Media.ReleaseDate))
			}

			message.WriteString(fmt.Sprintf("\n   📝 Added: %s\n\n",
				item.UserMedia.CreatedAt.Format("Jan 2, 2006")))
		}
	}

	if totalPages > 1 {
		message.WriteString("<i>💡 Use the navigation buttons below to browse through pages!</i>")
	}

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

// Message sending methods
func (h *Handler) sendMessage(ctx context.Context, chatID, text string) {
	h.sendMessageWithKeyboard(ctx, chatID, text, nil)
}

func (h *Handler) sendMessageWithKeyboard(ctx context.Context, chatID, text string, keyboard *models.InlineKeyboardMarkup) {
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		h.logger.WithError(err).Error("Invalid chat ID")
		return
	}

	if err := services.SendTelegramMessageWithKeyboard(ctx, h.botToken, chatIDInt, text, keyboard); err != nil {
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

func (h *Handler) editMessage(ctx context.Context, chatID string, messageID int, text string, keyboard *models.InlineKeyboardMarkup) {
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		h.logger.WithError(err).Error("Invalid chat ID for edit message")
		return
	}

	if err := services.EditTelegramMessage(ctx, h.botToken, chatIDInt, messageID, text, keyboard); err != nil {
		h.logger.WithFields(logrus.Fields{
			"chat_id":    chatIDInt,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to edit message")

		// Fallback: send new message if edit fails
		h.sendMessageWithKeyboard(ctx, chatID, text, keyboard)
	} else {
		h.logger.WithFields(logrus.Fields{
			"chat_id":    chatIDInt,
			"message_id": messageID,
		}).Debug("Message edited successfully")
	}
}

func (h *Handler) answerCallback(ctx context.Context, callbackID, text string, showAlert bool) {
	if err := services.AnswerCallbackQuery(ctx, h.botToken, callbackID, text, showAlert); err != nil {
		h.logger.WithFields(logrus.Fields{
			"callback_id": callbackID,
			"error":       err.Error(),
		}).Error("Failed to answer callback query")
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
