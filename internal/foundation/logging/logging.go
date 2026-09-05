package logging

import (
	"log/slog"
	"os"
)

type Config struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

func New(config Config) *slog.Logger {
	level := new(slog.LevelVar)
	switch config.Level {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	options := &slog.HandlerOptions{Level: level}
	if config.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
