package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sletish/internal/models"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	reminderCachePrefix = "reminder:user"
	reminderCacheTTL    = 10 * time.Minute
	workerInterval      = 5 * time.Minute
)

type ReminderService struct {
	db           *pgxpool.Pool
	redis        *redis.Client
	logger       *logrus.Logger
	botToken     string
	isRunning    bool
	animeService *Client // needed to ccreate media entries
}

type ReminderWorkerStats struct {
	LastRun            time.Time `json:"last_run"`
	RemindersProcessed int       `json:"reminders_processed"`
	NotificationsSent  int       `json:"notifications_sent"`
	Errors             int       `json:"errors"`
	IsRunning          bool      `json:"is_running"`
}

func NewReminderService(db *pgxpool.Pool, logger *logrus.Logger, redis *redis.Client, botToken string, animeService *Client) *ReminderService {
	service := &ReminderService{
		db:           db,
		logger:       logger,
		botToken:     botToken,
		animeService: animeService,
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

	// Check if media exists by external_id, create if it doesn't exist
	media, err := s.getOrCreateMediaByExternalID(mediaID)
	if err != nil {
		return fmt.Errorf("failed to get/create media: %w", err)
	}
	insertQuery := `
	INSERT INTO reminders (user_id, media_id, message, remind_at, sent, created_at)
	VALUES ($1, $2, $3, $4, false, $5)
	RETURNING id
	`
	var reminderID int
	err = s.db.QueryRow(context.Background(), insertQuery, userID, media.ID, message, remindAt, time.Now()).Scan(&reminderID)

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

func (s *ReminderService) getOrCreateMediaByExternalID(animeID int) (*models.Media, error) {
	query := `
    SELECT id, external_id, title, type, description, release_date, poster_url, rating, created_at
    FROM media
    WHERE external_id = $1
    `

	var media models.Media
	var releaseDate pgtype.Text
	var rating pgtype.Float8

	err := s.db.QueryRow(context.Background(), query, strconv.Itoa(animeID)).Scan(
		&media.ID,
		&media.ExternalID,
		&media.Title,
		&media.Type,
		&media.Description,
		&releaseDate,
		&media.PosterURL,
		&rating,
		&media.CreatedAt,
	)

	if err == nil {
		if releaseDate.Valid {
			media.ReleaseDate = &releaseDate.String
		}
		if rating.Valid {
			media.Rating = &rating.Float64
		}
		return &media, nil
	}

	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("database error: %w", err)
	}

	jikanAnime, err := s.animeService.GetAnimeByID(animeID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime from API: %w", err)
	}

	return s.createMediaFromJikan(*jikanAnime)
}

func (s *ReminderService) createMediaFromJikan(jikanAnime models.AnimeData) (*models.Media, error) {
	externalID := strconv.Itoa(jikanAnime.MalID)
	title := jikanAnime.Title
	description := jikanAnime.Synopsis
	releaseDate := ""
	posterURL := ""
	var rating *float64

	if jikanAnime.Score > 0 {
		rating = &jikanAnime.Score
	}
	if len(jikanAnime.Images.JPG.ImageURL) > 0 {
		posterURL = jikanAnime.Images.JPG.ImageURL
	}
	if len(description) > 1000 {
		description = description[:1000] + "..."
	}

	insertQuery := `
        INSERT INTO media (external_id, title, type, description, release_date, poster_url, rating, created_at)
        VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8)
        RETURNING id, external_id, title, type, description, release_date, poster_url, rating, created_at
    `

	var media models.Media
	var dbReleaseDate pgtype.Text
	var dbRating pgtype.Float8
	now := time.Now()

	err := s.db.QueryRow(context.Background(), insertQuery,
		externalID, title, "anime", description, releaseDate, posterURL, rating, now).Scan(
		&media.ID, &media.ExternalID, &media.Title, &media.Type, &media.Description,
		&dbReleaseDate, &media.PosterURL, &dbRating, &media.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert media: %w", err)
	}

	if dbReleaseDate.Valid {
		media.ReleaseDate = &dbReleaseDate.String
	}
	if dbRating.Valid {
		media.Rating = &dbRating.Float64
	}

	return &media, nil
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

func (s *ReminderService) GetUserReminders(userID string, includeSent bool) ([]models.Reminder, error) {
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

func (s *ReminderService) CancelReminder(userID string, reminderID int) error {
	deleteQuery := `
	DELETE FROM reminders
	WHERE id = $1
	AND user_id = $2
	AND sent = false
	`

	result, err := s.db.Exec(context.Background(), deleteQuery, reminderID, userID)
	if err != nil {
		return fmt.Errorf("failed to cancel reminder: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("reminder not found or already sent")
	}

	s.invalidateUserReminderCache(userID)

	s.logger.WithFields(logrus.Fields{
		"reminder_id": reminderID,
		"user_id":     userID,
	}).Info("Reminder cancelled successfully")

	return nil
}

func (s *ReminderService) GetWorkerStats() ReminderWorkerStats {
	return ReminderWorkerStats{
		IsRunning: s.isRunning,
		LastRun:   time.Now(),
	}
}

func (s *ReminderService) StopWorker() {
	s.isRunning = false
	s.logger.Info("Reminder worker stop requested")
}

func (s *ReminderService) SetBotToken(botToken string) {
	s.botToken = botToken
}
