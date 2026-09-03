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
	// SNMPMaxRepetitions is how many values one GETBULK asks an OLT for.
	SNMPMaxRepetitions int
	// WAMediaDir is where attachments from WhatsApp are written.
	WAMediaDir string
	// WASendIntervalMS is the gap left between two outgoing messages. Emptying
	// the queue at full speed is what gets an unofficial number flagged.
	WASendIntervalMS int
	// WADrainIntervalSeconds is how often the outbox is swept even when no Redis
	// announcement arrived, so a lost announcement costs latency, not a reply.
	WADrainIntervalSeconds int
	// WAMediaRetentionDays is how long an attachment is kept on disk.
	WAMediaRetentionDays int
	// FirebaseServiceAccountJSONB64 is the base64-encoded Firebase service
	// account key used to send push notifications. Empty means the feature is
	// not configured yet — cmd/api must still start normally (see
	// internal/push.NewClient).
	FirebaseServiceAccountJSONB64 string
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
	viper.SetDefault("SNMP_MAX_REPETITIONS", 10)
	viper.SetDefault("WA_MEDIA_DIR", "/data/cs-media")
	viper.SetDefault("WA_SEND_INTERVAL_MS", 1200)
	viper.SetDefault("WA_DRAIN_INTERVAL_SECONDS", 30)
	viper.SetDefault("WA_MEDIA_RETENTION_DAYS", 90)

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

		SNMPMaxRepetitions: viper.GetInt("SNMP_MAX_REPETITIONS"),

		WAMediaDir:             viper.GetString("WA_MEDIA_DIR"),
		WASendIntervalMS:       viper.GetInt("WA_SEND_INTERVAL_MS"),
		WADrainIntervalSeconds: viper.GetInt("WA_DRAIN_INTERVAL_SECONDS"),
		WAMediaRetentionDays:   viper.GetInt("WA_MEDIA_RETENTION_DAYS"),

		FirebaseServiceAccountJSONB64: viper.GetString("FIREBASE_SERVICE_ACCOUNT_JSON_B64"),
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
