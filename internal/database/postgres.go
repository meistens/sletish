package database

import (
	"fmt"
	"sletish/internal/config"
	"sletish/internal/logger"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/context"
)

var pool *pgxpool.Pool

func Init(ctx context.Context) error {
	host, port, user, password, databaseName := config.DatabaseConfig()

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, databaseName)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	config.HealthCheckPeriod = 5 * time.Minute

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

func MustInit(ctx context.Context) {
	if err := Init(ctx); err != nil {
		logger.Get().WithError(err).Fatal("Failed to initialize database connection")
	}
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
