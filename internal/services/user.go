package services

import (
	"context"
	// "database/sql"
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

// NewUserService creates and returns a new UserService.
// It requires a PostgreSQL connection pool, a Redis client, a logger, and an HTTP client.
func NewUserService(db *pgxpool.Pool, redis *redis.Client, logger *logrus.Logger, client *Client) *UserService {
	return &UserService{
		db:     db,
		redis:  redis,
		logger: logger,
		client: client,
	}
}

// EnsureUserExists checks whether a user exists in the database.
// If the user doesn't exist, it creates a new one. If the username has changed, it updates it.
// Also invalidates the user's cache.
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
		_, err := s.db.Exec(context.Background(), insertQuery, userID, username, now)
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

		_, err := s.db.Exec(context.Background(), updateQuery, userID, username)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	s.invalidateUserCache(userID)
	return nil
}

// GetUser retrieves a user profile by ID.
// If available, it attempts to fetch the user data from Redis cache.
// Falls back to the database if not cached, and caches the result.
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

// AddToUserList adds an anime (media) to a user's list with a specific status.
// If the anime is already in the user's list, it updates the status instead.
// Automatically fetches or creates the media entry from the Jikan API if not present in the DB.
// Invalidates user cache after the operation.
func (s *UserService) AddToUserList(userID string, animeID int, status models.Status) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"anime_id": animeID,
		"status":   status,
	}).Info("Adding anime to user list...")

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

	isNewEntry := false
	err = s.db.QueryRow(context.Background(), checkQuery, userID, media.ID).Scan(&existingAnimeID)

	if err != nil {
		if err == pgx.ErrNoRows {
			isNewEntry = true
		} else {
			return fmt.Errorf("failed to check existing user media: %w", err)
		}
	}

	now := time.Now()

	if isNewEntry {
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

// getOrCreateMediaByID tries to retrieve a media entry by its external ID (MyAnimeList ID).
// If it doesn't exist in the database, it fetches the data from the Jikan API and creates a new media record.
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

// getMediaByExternalID retrieves a media record from the database using its external (MyAnimeList) ID.
// Returns an error if not found.
func (s *UserService) getMediaByExternalID(externalID string) (*models.Media, error) {
	query := `
	SELECT id, external_id, title, type, description, release_date, poster_url, rating, created_at
	FROM media
	WHERE external_id = $1
	`

	var media models.Media
	var releaseDate pgtype.Text
	var rating pgtype.Float8

	err := s.db.QueryRow(context.Background(), query, externalID).Scan(
		&media.ID,
		&media.ExternalID,
		&media.Title,
		&media.Type,
		&media.Description,
		&releaseDate, // Handle NULL
		&media.PosterURL,
		&rating, // Handle NULL
		&media.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Convert NULL values
	if releaseDate.Valid {
		media.ReleaseDate = &releaseDate.String
	}
	if rating.Valid {
		media.Rating = &rating.Float64
	}

	return &media, nil
}

// createMediaFromJikan creates a new media record in the database using data fetched from the Jikan API.
// It stores the media in the database and caches the raw Jikan response in Redis.
func (s *UserService) createMediaFromJikan(jikanAnime models.AnimeData) (*models.Media, error) {
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

	// Insert media record
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
		&media.ID,
		&media.ExternalID,
		&media.Title,
		&media.Type,
		&media.Description,
		&dbReleaseDate, // Handle NULL return
		&media.PosterURL,
		&dbRating, // Handle NULL return
		&media.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert media: %w", err)
	}

	// Convert NULL values back
	if dbReleaseDate.Valid {
		media.ReleaseDate = &dbReleaseDate.String
	}
	if dbRating.Valid {
		media.Rating = &dbRating.Float64
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

// invalidateUserCache removes the user's cached profile from Redis, if caching is enabled.
// Used after any update to ensure fresh data is fetched on the next request.
func (s *UserService) invalidateUserCache(userID string) {
	if s.redis == nil {
		return
	}

	cacheKey := userCachePrefix + userID
	if err := s.redis.Del(context.Background(), cacheKey).Err(); err != nil {
		s.logger.WithError(err).Warn("Failed to invalidate user cache")
	}
}

// RemoveFromUserList deletes a media item from the user's list using the anime ID.
// Returns an error if the media does not exist in the user's list.
// Invalidates user cache after deletion.
func (s *UserService) RemoveFromUserList(userID string, animeID int) error {
	s.logger.WithFields(logrus.Fields{
		"user_id":  userID,
		"anime_id": animeID,
	}).Info("Removing anime from user list")

	media, err := s.getMediaByExternalID(strconv.Itoa(animeID))
	if err != nil {
		return fmt.Errorf("anime not found: %w", err)
	}

	// Delete user media record
	deleteQuery := `
	DELETE FROM user_media
	WHERE user_id = $1
	AND media_id = $2
	`

	result, err := s.db.Exec(context.Background(), deleteQuery, userID, media.ID)
	if err != nil {
		return fmt.Errorf("failed to delete user media: %w", err)
	}

	// check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("anime not found in user's list")
	}

	s.invalidateUserCache(userID)

	return nil
}

// UpdateAnimeStatus updates the status (e.g., watching, completed) of a specific anime in the user's list.
// Returns an error if the anime is not found in the user's list.
// Invalidates user cache after update.
func (s *UserService) UpdateAnimeStatus(userID string, animeID int, status models.Status) error {
	// fetch media record by external id
	media, err := s.getMediaByExternalID(strconv.Itoa(animeID))
	if err != nil {
		return fmt.Errorf("anime not found: %w", err)
	}

	query := `
		UPDATE user_media
		SET status = $1, updated_at = NOW()
		WHERE user_id = $2 AND media_id = $3
	`

	result, err := s.db.Exec(context.Background(), query, status, userID, media.ID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("anime not found in user's list")
	}

	s.invalidateUserCache(userID)

	return nil
}

func (s *UserService) contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// GetUserList retrieves all media entries from a user's list.
// Optionally filters by media status if a statusFilter is provided.
// Returns a slice of UserMediaWithDetails which includes both user-specific and media-specific data.
func (s *UserService) GetUserList(userID string, statusFilter string) ([]models.UserMediaWithDetails, error) {
	ctx, cancel := s.contextWithTimeout()
	defer cancel()

	query := `
		SELECT
			um.id, um.user_id, um.media_id, um.status, um.rating, um.notes, um.created_at, um.updated_at,
			m.id, m.external_id, m.title, m.type, m.description, m.release_date, m.poster_url, m.rating, m.created_at
		FROM user_media um
		JOIN media m ON um.media_id = m.id
		WHERE um.user_id = $1
	`

	args := []interface{}{userID}

	if statusFilter != "" {
		query += " AND um.status = $2"
		args = append(args, statusFilter)
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var list []models.UserMediaWithDetails

	for rows.Next() {
		var item models.UserMediaWithDetails
		// Use pgx nullable types for consistency
		var umRating pgtype.Float8
		var mRating pgtype.Float8
		var releaseDate pgtype.Text
		var notes pgtype.Text

		err := rows.Scan(
			// UserMedia fields
			&item.UserMedia.ID,
			&item.UserMedia.UserID,
			&item.UserMedia.MediaID,
			&item.UserMedia.Status,
			&umRating,
			&notes,
			&item.UserMedia.CreatedAt,
			&item.UserMedia.UpdatedAt,

			// Media fields
			&item.Media.ID,
			&item.Media.ExternalID,
			&item.Media.Title,
			&item.Media.Type,
			&item.Media.Description,
			&releaseDate,
			&item.Media.PosterURL,
			&mRating,
			&item.Media.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Assign values from pgx nullable types
		if umRating.Valid {
			item.UserMedia.Rating = umRating.Float64
		}
		if notes.Valid {
			item.UserMedia.Notes = notes.String
		}

		if mRating.Valid {
			item.Media.Rating = &mRating.Float64
		}

		if releaseDate.Valid {
			item.Media.ReleaseDate = &releaseDate.String
		}

		list = append(list, item)
	}

	return list, nil
}
