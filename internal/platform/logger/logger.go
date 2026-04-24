// internal/platform/logger/logger.go
package logger

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg config.LoggerConfig) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	zcfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: cfg.Level == "debug",
		Encoding:    cfg.Format,
		OutputPaths: []string{cfg.Output},
		ErrorOutputPaths: []string{
			cfg.Output,
		},
		EncoderConfig: zap.NewProductionEncoderConfig(),
	}

	return zcfg.Build()
}
