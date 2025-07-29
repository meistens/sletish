package container

import (
	"context"
	"fmt"
	"sletish/internal/config"
	"sletish/internal/logger"
	"sletish/internal/services"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type Container struct {
	DB           *pgxpool.Pool
	Redis        *redis.Client
	Logger       *logrus.Logger
	AnimeService *services.Client
	UserService  *services.UserService
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
		DB:           db,
		Redis:        redisClient,
		Logger:       logger,
		AnimeService: services.NewClientWithConfig(animeConfig),
		UserService:  services.NewUserService(db, redisClient, logger),
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
	host, port, user, password, databaseName := config.DatabaseConfig()

	if host == "" || port == "" || user == "" || password == "" || databaseName == "" {
		return nil, fmt.Errorf("missing required database configuration")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, databaseName)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Better pool configuration
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
	host, port, password := config.RedisConfig()

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Get().Info("Redis connection successful")
	return client, nil
}
