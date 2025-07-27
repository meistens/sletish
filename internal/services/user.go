package services

import (
	"context"
	"fmt"
	"sletish/internal/database"
	"sletish/internal/logger"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	userCachePrefix = "user:info:"
	userCacheTTL    = 30 * time.Minute
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) EnsureUserExists(userID, username string) error {
	db := database.Get()
	log := logger.Get()

	log.WithFields(logrus.Fields{
		"user_id":  userID,
		"username": username,
	}).Info("Checking if user exists...")

	var exists bool
	err := db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	now := time.Now()

	if !exists {
		insertQuery := `
		INSERT INTO users (id, username, platform, created_at, updated_at)
		VALUES ($1, $2, 'telegram', $3, $3)
		`
		_, err := db.Exec(context.Background(), insertQuery, userID, username, now)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		log.WithFields(logrus.Fields{
			"user_id":  userID,
			"username": username,
		}).Info("A user has been created...")
	} else {
		updateQuery := `
		UPDATE users
		SET username = $2
		WHERE id = $1 AND (username IS NULL OR username != $2)
		`

		_, err := db.Exec(context.Background(), updateQuery, userID, username)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	return nil
}
