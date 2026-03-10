package config

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger initializes a Zap logger based on the provided log level.
func InitLogger(levelStr string) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		level = zap.InfoLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	cfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       false,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig:     encoderConfig,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// For local development, we can switch to console encoding if needed
	// if os.Getenv("ENV") == "dev" {
	// 	cfg.Encoding = "console"
	// }

	return cfg.Build()
}
