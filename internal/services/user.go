package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sletish/internal/models"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	userCachePrefix  = "user:info:"
	userCacheTTL     = 30 * time.Minute
	animeCachePrefix = "anime:details:"
	animeCacheTTL    = 1 * time.Hour
)

type UserService struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *logrus.Logger
	client *Client
}

func NewUserService(db *pgxpool.Pool, redis *redis.Client, logger *logrus.Logger, client *Client) *UserService {
	return &UserService{
		db:     db,
		redis:  redis,
		logger: logger,
		client: client,
	}
}

func (s *UserService) EnsureUserExists(userID, username string) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"username": username,
	}).Info("Checking if user exists...")

	var exists bool
	err := s.db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	now := time.Now()

	if !exists {
		insertQuery := `
		INSERT INTO users (id, username, platform, created_at, updated_at)
		VALUES ($1, $2, 'telegram', $3, $3)
		`
		var usernamePtr *string
		if username != "" {
			usernamePtr = &username
		}

		_, err := s.db.Exec(context.Background(), insertQuery, userID, usernamePtr, now)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"user_id":  userID,
			"username": username,
		}).Info("A user has been created...")
	} else {
		updateQuery := `
		UPDATE users
		SET username = $2
		WHERE id = $1 AND (username IS NULL OR username != $2)
		`

		var usernamePtr *string
		if username != "" {
			usernamePtr = &username
		}

		_, err := s.db.Exec(context.Background(), updateQuery, userID, usernamePtr)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	s.invalidateUserCache(userID)
	return nil
}

func (s *UserService) GetUser(userID string) (*models.AppUser, error) {
	if s.redis != nil {
		cacheKey := userCachePrefix + userID

		cached, err := s.redis.Get(context.Background(), cacheKey).Result()

		if err == nil {
			s.logger.WithField("user_id", userID).Debug("Retrieved user from cache")

			var cachedUser models.AppUser
			if err := json.Unmarshal([]byte(cached), &cachedUser); err == nil {
				return &cachedUser, nil
			}

			s.logger.WithError(err).Warn("Failed to unmarshal cached user")
		} else if err != redis.Nil {
			s.logger.WithError(err).Warn("Failed to read from Redis")
		}
	}

	// get from db
	getQuery := `
		SELECT id, username, platform, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var user models.AppUser
	err := s.db.QueryRow(context.Background(), getQuery, userID).Scan(&user.ID,
		&user.Username,
		&user.Platform,
		&user.CreatedAt,
		&user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if s.redis != nil {
		cacheKey := userCachePrefix + userID
		userJSON, err := json.Marshal(user)
		if err == nil {
			if err := s.redis.Set(context.Background(), cacheKey, userJSON, userCacheTTL).Err(); err != nil {
				s.logger.WithError(err).Warn("Failed to cache user")
			}
		}
	}

	return &user, nil
}

func (s *UserService) AddToUserList(userID string, animeID int, status models.Status) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"anime_id": animeID,
		"status":   status,
	}).Info("Adding anime to user list...")

	// check if anime exists in the API
	_, err := s.client.GetAnimeByID(animeID)
	if err != nil {
		return fmt.Errorf("anime with ID %d not found: %w", animeID, err)
	}

	media, err := s.getOrCreateMediaByID(animeID)
	if err != nil {
		return fmt.Errorf("failed to get/create media: %w", err)
	}

	// check if user has anime on their list
	var existingAnimeID int
	checkQuery := `
	SELECT id
	FROM user_media
	WHERE user_id = $1
	AND media_id = $2
	`

	err = s.db.QueryRow(context.Background(), checkQuery, userID, media.ID).Scan(&existingAnimeID)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("failed to check existing user media: %w", err)
	}

	now := time.Now()

	if err == pgx.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO user_media (user_id, media_id, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			`

		_, err = s.db.Exec(context.Background(), insertQuery, userID, media.ID, status, now)
		if err != nil {
			return fmt.Errorf("failed to insert user media: %w", err)
		}
		s.logger.Info("Added anime to user list")
	} else {
		// Update existing record
		updateQuery := `
			UPDATE user_media
			SET status = $3, updated_at = $4
			WHERE user_id = $1 AND media_id = $2
			`

		_, err = s.db.Exec(context.Background(), updateQuery, userID, media.ID, status, now)

		if err != nil {
			return fmt.Errorf("failed to update user media: %w", err)
		}
		s.logger.Info("Updated anime status in user list")
	}

	s.invalidateUserCache(userID)
	return nil
}

func (s *UserService) RemoveFromUserList(userID string, animeID int) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"anime_id": animeID,
	}).Info("Removing anime from user list...")

	media, err := s.getMediaByExternalID(strconv.Itoa(animeID))
	if err != nil {
		return fmt.Errorf("anime not found in database: %w", err)
	}

	deleteQuery := `
		DELETE FROM user_media
		WHERE user_id = $1 AND media_id = $2
	`

	result, err := s.db.Exec(context.Background(), deleteQuery, userID, media.ID)
	if err != nil {
		return fmt.Errorf("failed to remove anime from user list: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("anime not found in your list")
	}

	s.logger.Info("Removed anime from user list")
	s.invalidateUserCache(userID)
	return nil
}

func (s *UserService) GetUserList(userID string, status models.Status) ([]models.UserMediaWithDetails, error) {
	s.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"status":  status,
	}).Info("Getting user anime list...")

	query := `
		SELECT um.id, um.user_id, um.media_id, um.status, um.rating, um.notes, um.created_at, um.updated_at,
		       m.id, m.external_id, m.title, m.type, m.description, m.release_date, m.poster_url, m.rating, m.created_at
		FROM user_media um
		JOIN media m ON um.media_id = m.id
		WHERE um.user_id = $1
	`

	args := []interface{}{userID}

	if status != "" {
		query += " AND um.status = $2"
		args = append(args, status)
	}

	query += " ORDER BY um.updated_at DESC"

	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get user list: %w", err)
	}
	defer rows.Close()

	var results []models.UserMediaWithDetails
	for rows.Next() {
		var userMedia models.UserMedia
		var media models.Media

		err := rows.Scan(
			&userMedia.ID, &userMedia.UserID, &userMedia.MediaID, &userMedia.Status,
			&userMedia.Rating, &userMedia.Notes, &userMedia.CreatedAt, &userMedia.UpdatedAt,
			&media.ID, &media.ExternalID, &media.Title, &media.Type,
			&media.Description, &media.ReleaseDate, &media.PosterURL, &media.Rating, &media.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user media: %w", err)
		}

		results = append(results, models.UserMediaWithDetails{
			UserMedia: userMedia,
			Media:     media,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *UserService) UpdateAnimeStatus(userID string, animeID int, status models.Status) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"anime_id": animeID,
		"status":   status,
	}).Info("Updating anime status...")

	media, err := s.getMediaByExternalID(strconv.Itoa(animeID))
	if err != nil {
		return fmt.Errorf("anime not found in database: %w", err)
	}

	updateQuery := `
		UPDATE user_media
		SET status = $3, updated_at = $4
		WHERE user_id = $1 AND media_id = $2
	`

	result, err := s.db.Exec(context.Background(), updateQuery, userID, media.ID, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update anime status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("anime not found in your list")
	}

	s.logger.Info("Updated anime status")
	s.invalidateUserCache(userID)
	return nil
}

func (s *UserService) getOrCreateMediaByID(animeID int) (*models.Media, error) {
	media, err := s.getMediaByExternalID(strconv.Itoa(animeID))
	if err == nil {
		return media, nil
	}

	// fetch from API if no anime is found on list
	jikanAnime, err := s.client.GetAnimeByID(animeID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime from Jikan: %w", err)
	}

	// create record
	return s.createMediaFromJikan(*jikanAnime)
}

func (s *UserService) getMediaByExternalID(externalID string) (*models.Media, error) {
	query := `
	SELECT id, external_id, title, type, description, release_date, poster_url, rating, created_at
	FROM media
	WHERE external_id = $1
	`

	var media models.Media
	err := s.db.QueryRow(context.Background(), query, externalID).Scan(
		&media.ID, &media.ExternalID, &media.Title, &media.Type, &media.Description,
		&media.ReleaseDate, &media.PosterURL, &media.Rating, &media.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &media, nil
}

func (s *UserService) createMediaFromJikan(jikanAnime models.AnimeData) (*models.Media, error) {
	externalID := strconv.Itoa(jikanAnime.MalID)
	title := jikanAnime.Title
	var description *string
	var releaseDate *string
	var posterURL *string
	var rating *float64

	if jikanAnime.Score > 0 {
		rating = &jikanAnime.Score
	}
	if jikanAnime.Images.JPG.ImageURL != "" {
		posterURL = &jikanAnime.Images.JPG.ImageURL
	}
	if jikanAnime.Synopsis != "" {
		desc := jikanAnime.Synopsis
		if len(desc) > 1000 {
			desc = desc[:1000] + "..."
		}
		description = &desc
	}
	if jikanAnime.Year > 0 {
		yearStr := strconv.Itoa(jikanAnime.Year)
		releaseDate = &yearStr
	}

	// Insert media record
	insertQuery := `
		INSERT INTO media (external_id, title, type, description, release_date, poster_url, rating, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, external_id, title, type, description, release_date, poster_url, rating, created_at
	`

	var media models.Media
	now := time.Now()

	err := s.db.QueryRow(context.Background(), insertQuery, externalID, title, "anime", description, releaseDate, posterURL, rating, now).Scan(
		&media.ID, &media.ExternalID, &media.Title, &media.Type, &media.Description,
		&media.ReleaseDate, &media.PosterURL, &media.Rating, &media.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert media: %w", err)
	}

	// Cache anime details
	if s.redis != nil {
		cacheKey := animeCachePrefix + externalID
		animeJSON, err := json.Marshal(jikanAnime)
		if err == nil {
			s.redis.Set(context.Background(), cacheKey, animeJSON, animeCacheTTL)
		}
	}

	return &media, nil
}

func (s *UserService) invalidateUserCache(userID string) {
	if s.redis == nil {
		return
	}

	cacheKey := userCachePrefix + userID
	if err := s.redis.Del(context.Background(), cacheKey).Err(); err != nil {
		s.logger.WithError(err).Warn("Failed to invalidate user cache")
	}
}
