package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host                     string
	Port                     string
	APIGatewayPort           string
	MessageEnginePort        string
	DatabaseURL              string
	RedisURL                 string
	JWTSecret                string
	JWTExpiry                time.Duration
	AutoMigrate              bool
	RateLimitRequests        int
	RateLimitWindow          time.Duration
	OTELEnabled              bool
	OTELExporterOTLPEndpoint string
	OTELInsecure             bool
	LogLevel                 string
	Env                      string
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Host:                     getEnv("HOST", "0.0.0.0"),
		Port:                     getEnv("PORT", "8080"),
		APIGatewayPort:           getEnv("API_GATEWAY_PORT", "8080"),
		MessageEnginePort:        getEnv("MESSAGE_ENGINE_PORT", "8081"),
		DatabaseURL:              getEnv("DATABASE_URL", ""),
		RedisURL:                 getEnv("REDIS_URL", ""),
		JWTSecret:                getEnv("JWT_SECRET", "dev-secret"),
		AutoMigrate:              getEnvBool("AUTO_MIGRATE", false),
		RateLimitRequests:        getEnvInt("RATE_LIMIT_REQUESTS", 600),
		RateLimitWindow:          time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
		OTELEnabled:              getEnvBool("OTEL_ENABLED", false),
		OTELExporterOTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELInsecure:             getEnvBool("OTEL_INSECURE", true),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
		Env:                      getEnv("ENV", "development"),
		JWTExpiry:                24 * time.Hour,
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "postgres://agentmsg:agentmsg@localhost:5432/agentmsg?sslmode=disable"
	}
	if cfg.RedisURL == "" {
		cfg.RedisURL = "redis://localhost:6379/0"
	}

	return cfg, nil
}

func (c *Config) Validate(requireJWTSecret bool) error {
	if c == nil {
		return fmt.Errorf("invalid config: config is nil")
	}

	var issues []string

	if c.RateLimitRequests <= 0 {
		issues = append(issues, "RATE_LIMIT_REQUESTS must be greater than 0")
	}
	if c.RateLimitWindow <= 0 {
		issues = append(issues, "RATE_LIMIT_WINDOW_SECONDS must be greater than 0")
	}
	if c.OTELEnabled && strings.TrimSpace(c.OTELExporterOTLPEndpoint) == "" {
		issues = append(issues, "OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}
	if requireJWTSecret {
		if strings.TrimSpace(c.JWTSecret) == "" {
			issues = append(issues, "JWT_SECRET is required")
		}
		if isProductionEnv(c.Env) && isInsecureJWTSecret(c.JWTSecret) {
			issues = append(issues, "JWT_SECRET must be changed from development defaults in production")
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(issues, "; "))
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func isInsecureJWTSecret(secret string) bool {
	normalized := strings.ToLower(strings.TrimSpace(secret))
	switch {
	case normalized == "":
		return true
	case normalized == "dev-secret":
		return true
	case strings.Contains(normalized, "change-me"):
		return true
	case strings.Contains(normalized, "change-in-production"):
		return true
	default:
		return false
	}
}
