// Package config reads service configuration from the environment with defaults.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func String(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func Int(key string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return def
}

func Bool(key string, def bool) bool {
	if v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return def
}

func Duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return def
}

// Common values every service reads.
func RabbitURL() string  { return String("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/") }
func JWTSecret() string  { return String("JWT_SECRET", "dev-only-change-me") }
func ServiceKey() string { return String("SERVICE_KEY", "dev-service-key") }
func SeedDir() string    { return String("SEED_DIR", "../../tools/seed/out") }
func RedisURL() string   { return String("REDIS_URL", "") }
