package cache

import (
	"context"
	"fmt"
	"sletish/internal/config"
	"sletish/internal/logger"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func Init() {
	host, port, password := config.RedisConfig()

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})

	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Get().WithError(err).Fatal("Failed to connect to Redis")
	}
	logger.Get().Info("Connection to Redis successful")
}

func Get() *redis.Client {
	return redisClient
}

func Close() {
	if redisClient != nil {
		redisClient.Close()
	}
}
