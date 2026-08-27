package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	DBHost         string
	DBPort         int
	DBUser         string
	DBPassword     string
	DBName         string
	RedisHost      string
	RedisPort      int
	RedisPassword  string
	EncryptionKey  string
	SessionSecret  string
	LogLevel       string
	APIPort        int
	Environment    string
	AllowedOrigins string
}

func Load() (*Config, error) {
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)
	viper.SetDefault("DB_USER", "tikman")
	viper.SetDefault("DB_NAME", "tikman")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", 6379)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("API_PORT", 8080)
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("ALLOWED_ORIGINS", "http://localhost:3000")

	viper.AutomaticEnv()

	cfg := &Config{
		DBHost:         viper.GetString("DB_HOST"),
		DBPort:         viper.GetInt("DB_PORT"),
		DBUser:         viper.GetString("DB_USER"),
		DBPassword:     viper.GetString("DB_PASSWORD"),
		DBName:         viper.GetString("DB_NAME"),
		RedisHost:      viper.GetString("REDIS_HOST"),
		RedisPort:      viper.GetInt("REDIS_PORT"),
		RedisPassword:  viper.GetString("REDIS_PASSWORD"),
		EncryptionKey:  viper.GetString("ENCRYPTION_KEY"),
		SessionSecret:  viper.GetString("SESSION_SECRET"),
		LogLevel:       viper.GetString("LOG_LEVEL"),
		APIPort:        viper.GetInt("API_PORT"),
		Environment:    viper.GetString("ENVIRONMENT"),
		AllowedOrigins: viper.GetString("ALLOWED_ORIGINS"),
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if cfg.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if len(cfg.EncryptionKey) != 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be 32 bytes")
	}
	if cfg.SessionSecret == "" {
		return fmt.Errorf("SESSION_SECRET is required")
	}
	return nil
}
