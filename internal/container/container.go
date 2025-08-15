package container

import (
	"context"
	"fmt"
	"os"
	"sletish/internal/logger"
	"sletish/internal/services"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type Container struct {
	DB              *pgxpool.Pool
	Redis           *redis.Client
	Logger          *logrus.Logger
	AnimeService    *services.Client
	UserService     *services.UserService
	ReminderService *services.ReminderService
}

func New(ctx context.Context) (*Container, error) {
	// Initialize logger first
	logger := logger.Get()

	// Initialize database
	db, err := newDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis
	redisClient, err := newRedis(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize redis: %w", err)
	}

	// Initialize services
	animeConfig := &services.ClientConfig{
		BaseURL:    "https://api.jikan.moe/v4",
		Timeout:    30 * time.Second,
		RateLimit:  1 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		UserAgent:  "AnimeTrackerBot/1.0",
		Logger:     logger,
		Redis:      redisClient,
	}

	return &Container{
		DB:              db,
		Redis:           redisClient,
		Logger:          logger,
		AnimeService:    services.NewClientWithConfig(animeConfig),
		UserService:     services.NewUserService(db, redisClient, logger, services.NewClient()),
		ReminderService: services.NewReminderService(db, logger, redisClient, "", services.NewClientWithConfig(animeConfig)),
	}, nil
}

func (c *Container) Close() {
	if c.Redis != nil {
		c.Redis.Close()
		c.Logger.Info("Redis connection closed")
	}
	if c.DB != nil {
		c.DB.Close()
		c.Logger.Info("Database connection closed")
	}
}

func newDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = time.Minute * 30
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	logger.Get().Info("Database connection successful")
	return pool, nil
}

func newRedis(ctx context.Context) (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	logger.Get().Info("Redis connection successful")
	return client, nil
}
