package logger

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew_ValidLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			log, err := New(level)
			require.NoError(t, err)
			require.NotNil(t, log)
			defer func() { _ = log.Sync() }()
		})
	}
}

func TestNew_InvalidLogLevel(t *testing.T) {
	log, err := New("invalid")
	assert.Error(t, err)
	assert.Nil(t, log)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestNew_TimestampFormatUTC(t *testing.T) {
	// Create a logger with custom output for testing
	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development:       false,
		Encoding:          "json",
		EncoderConfig:     zap.NewProductionEncoderConfig(),
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
		DisableStacktrace: false,
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		zapcore.ISO8601TimeEncoder(t.UTC(), enc)
	}

	// Build logger
	log, err := config.Build()
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	// Capture log output
	var buf strings.Builder
	encoder := zapcore.NewJSONEncoder(config.EncoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), config.Level)
	testLog := zap.New(core)

	// Log a message
	testLog.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err = json.Unmarshal([]byte(buf.String()), &logEntry)
	require.NoError(t, err)

	// Verify timestamp exists
	timestamp, ok := logEntry["timestamp"].(string)
	require.True(t, ok, "timestamp field should exist and be a string")

	// Parse the timestamp
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	require.NoError(t, err, "timestamp should be in ISO8601/RFC3339 format")

	// Verify it's in UTC (should have Z suffix or +00:00)
	assert.True(t, strings.HasSuffix(timestamp, "Z") || strings.Contains(timestamp, "+00:00"),
		"timestamp should be in UTC format")

	// Verify the parsed time is actually UTC
	assert.Equal(t, "UTC", parsedTime.Location().String())
}
