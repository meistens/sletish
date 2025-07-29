package repository

import (
	"context"
	"sletish/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.AppUser) error
	GetByID(ctx context.Context, id string) (*models.AppUser, error)
	Update(ctx context.Context, user *models.AppUser) error
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}
