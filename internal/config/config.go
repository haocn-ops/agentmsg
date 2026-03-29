package config

import (
	"os"
	"time"
)

type Config struct {
	Host              string
	Port              string
	APIGatewayPort    string
	MessageEnginePort string
	DatabaseURL       string
	RedisURL          string
	JWTSecret         string
	JWTExpiry         time.Duration
	LogLevel          string
	Env               string
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Host:              getEnv("HOST", "0.0.0.0"),
		Port:              getEnv("PORT", "8080"),
		APIGatewayPort:    getEnv("API_GATEWAY_PORT", "8080"),
		MessageEnginePort: getEnv("MESSAGE_ENGINE_PORT", "8081"),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		RedisURL:          getEnv("REDIS_URL", ""),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		Env:               getEnv("ENV", "development"),
		JWTExpiry:         24 * time.Hour,
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "postgres://agentmsg:agentmsg@localhost:5432/agentmsg?sslmode=disable"
	}
	if cfg.RedisURL == "" {
		cfg.RedisURL = "redis://localhost:6379/0"
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
