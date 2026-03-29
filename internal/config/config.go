package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
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
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		viper.SetDefault("HOST", "0.0.0.0")
		viper.SetDefault("PORT", "8080")
		viper.SetDefault("API_GATEWAY_PORT", "8080")
		viper.SetDefault("MESSAGE_ENGINE_PORT", "8081")
		viper.SetDefault("LOG_LEVEL", "info")
		viper.SetDefault("ENV", "development")
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
	 return nil, err
	}

	return &cfg, nil
}
