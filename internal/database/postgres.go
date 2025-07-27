package database

import (
	"context"
	"fmt"
	"sletish/internal/config"
	"sletish/internal/logger"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func Init(ctx context.Context) error {
	host, port, user, password, databaseName := config.DatabaseConfig()

	if host == "" || port == "" || user == "" || password == "" || databaseName == "" {
		return fmt.Errorf("missing required database configuration")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, databaseName)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	// Better pool configuration
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = time.Minute * 30
	config.HealthCheckPeriod = time.Minute

	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Get().Info("Connection to database successful!")
	return nil
}

func MustInit(ctx context.Context) error {
	return Init(ctx)
}

func Get() *pgxpool.Pool {
	return pool
}

func Close() {
	if pool != nil {
		pool.Close()
		logger.Get().Info("Connection pool is closed")
	}
}
