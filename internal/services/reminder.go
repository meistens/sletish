package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sletish/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	reminderCachePrefix = "reminder:user"
	reminderCacheTTL    = 10 * time.Minute
	workerInterval      = 30 * time.Minute
)

type ReminderService struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	logger    *logrus.Logger
	botToken  string
	isRunning bool
}

type ReminderWorkerStats struct {
	LastRun            time.Time `json:"last_run"`
	RemindersProcessed int       `json:"reminders_processed"`
	NotificationsSent  int       `json:"notifications_sent"`
	Errors             int       `json:"errors"`
	IsRunning          bool      `json:"is_running"`
}

func NewReminderService(db *pgxpool.Pool, logger *logrus.Logger) *ReminderService {
	return &ReminderService{
		db:       db,
		logger:   logger,
		botToken: "",
	}

	// start worker
	go service.StartReminderWorker()

	return service
}

func (s *ReminderService) StartReminderWorker() {
	s.logger.Info("Starting reminder worker...")
	s.isRunning = true

	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()

	for range ticker.C {
		if !s.isRunning {
			break
		}

		s.logger.Debug("Checking for due reminders...")

		if err := s.processDueReminders(); err != nil {
			s.logger.WithError(err).Error("Error processing due reminders")
		}
	}

	s.logger.Info("Reminder worker stopped")
}

func (s *ReminderService) processDueReminders() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	query := `
        SELECT r.id, r.user_id, r.media_id, r.message, r.remind_at, m.title, m.external_id
        FROM reminders r
        JOIN media m ON r.media_id = m.id
        WHERE r.sent = false AND r.remind_at <= $1
        ORDER BY r.remind_at ASC
        LIMIT 50
    `

	rows, err := s.db.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to query due reminders: %w", err)
	}
	defer rows.Close()

	var processedCount int
	var errorCount int

	for rows.Next() {
		var reminder = &models.Reminder{} // using the struct fields that matter instead of rewriting the damn thing
		err := rows.Scan(reminder.ID, reminder.UserID, reminder.MediaID, reminder.Message, reminder.RemindAt, reminder.MediaTitle, reminder.ExternalID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to scan reminder row")
			errorCount++
			continue
		}

		if err := s.sendReminderNotification(ctx, reminder.UserID, reminder.MediaTitle, reminder.ExternalID, reminder.Message, reminder.RemindAt); err != nil {
			s.logger.WithError(err).Error("Failed to send reminder notification")
			errorCount++
			continue
		}

		if err := s.markReminderAsSent(ctx, reminder.ID); err != nil {
			s.logger.WithError(err).Error("Failed to mark reminder as sent")
			errorCount++
			continue
		}

		processedCount++
		s.logger.WithFields(logrus.Fields{
			"reminder_id": reminder.ID,
			"user_id":     reminder.UserID,
		}).Info("Reminder sent successfully")
	}

	if processedCount > 0 || errorCount > 0 {
		s.logger.WithFields(logrus.Fields{
			"processed": processedCount,
			"errors":    errorCount,
		}).Info("Processed due reminders")
	}

	return nil
}

func (s *ReminderService) sendReminderNotification(ctx context.Context, userID, mediaTitle, externalID, message string, remindAt time.Time) error {
	chatID, err := strconv.Atoi(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	notificationText := fmt.Sprintf(`🔔 <b>Reminder!</b>

🎬 <b>%s</b>
💬 "%s"

⏰ <i>You set this reminder for %s</i>

<a href="https://myanimelist.net/anime/%s">🔗 View on MyAnimeList</a>`,
		mediaTitle, message, remindAt.Format("January 2, 2006"), externalID)

	return SendTelegramMessage(ctx, s.botToken, chatID, notificationText)
}

func (s *ReminderService) markReminderAsSent(ctx context.Context, reminderID int) error {
	updateQuery := `
	UPDATE reminders
	SET sent = true
	WHERE id = $1
	`

	_, err := s.db.Exec(ctx, updateQuery, reminderID)
	if err != nil {
		return fmt.Errorf("failed to mark reminder as sent: %w", err)
	}

	return nil
}

func (s *ReminderService) CreateReminder(userID string, mediaID int, message string, remindAt time.Time) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":   userID,
		"media_id":  mediaID,
		"remind_at": remindAt,
	}).Info("Creating reminder...")

	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if mediaID <= 0 {
		return fmt.Errorf("invalid media ID: %d", mediaID)
	}
	if message == "" {
		return fmt.Errorf("reminder message cannot be empty")
	}
	if remindAt.Before(time.Now()) {
		return fmt.Errorf("reminder time cannot be in the past")
	}

	var mediaExists bool
	err := s.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM media WHERE external_id = $1)", strconv.Itoa(mediaID)).Scan(&mediaExists)
	if err != nil {
		return fmt.Errorf("failed to check media existence: %w", err)
	}
	if !mediaExists {
		return fmt.Errorf("media with ID %d does not exist", mediaID)
	}

	insertQuery := `
    INSERT INTO reminders (user_id, media_id, message, remind_at, sent, created_at)
    SELECT $1, m.id, $3, $4, false, $5
    FROM media m
    WHERE m.external_id = $2
    RETURNING id
    `
	var reminderID int
	err = s.db.QueryRow(context.Background(), insertQuery, userID, strconv.Itoa(mediaID), message, remindAt, time.Now()).Scan(&reminderID)
	if err != nil {
		return fmt.Errorf("failed to create reminder: %w", err)
	}

	s.invalidateUserReminderCache(userID)

	s.logger.WithFields(logrus.Fields{
		"reminder_id": reminderID,
		"user_id":     userID,
		"media_id":    mediaID,
	}).Info("Reminder created successfully")

	return nil
}

func (s *ReminderService) invalidateUserReminderCache(userID string) {
	if s.redis == nil {
		return
	}

	patterns := []string{
		reminderCachePrefix + userID,
		reminderCachePrefix + userID + ":*",
	}

	for _, pattern := range patterns {
		keys, err := s.redis.Keys(context.Background(), pattern).Result()
		if err != nil {
			continue
		}

		if len(keys) > 0 {
			s.redis.Del(context.Background(), keys...)
		}
	}
}

func (s *ReminderService) GetUserReminder(userID string, includeSent bool) ([]models.Reminder, error) {
	s.logger.WithFields(logrus.Fields{
		"user_id":      userID,
		"include_sent": includeSent,
	}).Debug("Getting user reminders")

	cacheKey := reminderCachePrefix + userID
	if !includeSent {
		cacheKey += ":pending"
	}

	if s.redis != nil {
		cached, err := s.redis.Get(context.Background(), cacheKey).Result()
		if err != nil {
			s.logger.WithField("user_id", userID).Debug("Retrieved reminders from cache")

			var cachedReminders []models.Reminder
			if err := json.Unmarshal([]byte(cached), &cachedReminders); err == nil {
				return cachedReminders, nil
			}
		}
	}

	query := `
		SELECT r.id, r.user_id, r.media_id, r.message, r.remind_at, r.sent, r.created_at,
			   m.title, m.poster_url
		FROM reminders r
		JOIN media m ON r.media_id = m.id
		WHERE r.user_id = $1
`

	args := []interface{}{userID}
	if !includeSent {
		query += " AND r.sent = false"
	}

	query += " ORDER BY r.remind_at ASC"

	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed t query reminders: %w", err)
	}
	defer rows.Close()

	var reminders []models.Reminder
	for rows.Next() {
		var reminder models.Reminder
		var mediaTitle, posterURL pgtype.Text

		err := rows.Scan(
			&reminder.ID, &reminder.UserID, &reminder.MediaID, &reminder.Message,
			&reminder.RemindAt, &reminder.Sent, &reminder.CreatedAt,
			&mediaTitle, &posterURL,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan reminder row: %w", err)
		}

		if mediaTitle.Valid {
			reminder.MediaTitle = mediaTitle.String
		}
		if posterURL.Valid {
			reminder.MediaPosterURL = posterURL.String
		}

		reminders = append(reminders, reminder)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reminder rows: %w", err)
	}

	if s.redis != nil {
		remindersJSON, err := json.Marshal(reminders)
		if err == nil {
			s.redis.Set(context.Background(), cacheKey, remindersJSON, reminderCacheTTL)
		}
	}

	return reminders, nil
}
