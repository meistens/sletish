package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sletish/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	userCachePrefix = "user:info:"
	userCacheTTL    = 30 * time.Minute
)

type UserService struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *logrus.Logger
}

func NewUserService(db *pgxpool.Pool, redis *redis.Client, logger *logrus.Logger) *UserService {
	return &UserService{
		db:     db,
		redis:  redis,
		logger: logger,
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

func (s *UserService) invalidateUserCache(userID string) {
	if s.redis == nil {
		return
	}

	cacheKey := userCachePrefix + userID
	if err := s.redis.Del(context.Background(), cacheKey).Err(); err != nil {
		s.logger.WithError(err).Warn("Failed to invalidate user cache")
	}
}
