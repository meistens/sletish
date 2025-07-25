package config

import "os"

// RedisConfig returns host, port, password
func RedisConfig() (string, string, string) {
	host := GetEnv("R_HOST", "redis")
	port := GetEnv("R_HOST", "6379")
	password := GetEnv("R_PASS", "")
	return host, port, password
}

// GetEnv retrieves values from environment files based on the key it matches,
// returns a string (value) if not empty
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
