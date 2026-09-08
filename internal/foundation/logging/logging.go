package logging

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Config struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

func New(config Config) *slog.Logger {
	return newLogger(config, os.Stdout, terminalColorsEnabled(os.Stdout))
}

func newLogger(config Config, output io.Writer, colors bool) *slog.Logger {
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
		if colors {
			output = &colorWriter{output: output}
		}
		return slog.New(slog.NewTextHandler(output, options))
	}
	return slog.New(slog.NewJSONHandler(output, options))
}

func terminalColorsEnabled(output *os.File) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := output.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type colorWriter struct {
	output io.Writer
	mu     sync.Mutex
}

func (w *colorWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	colored := append([]byte(nil), data...)
	for _, replacement := range levelColors {
		colored = bytes.Replace(colored, replacement.plain, replacement.colored, 1)
	}
	written, err := w.output.Write(colored)
	if err != nil {
		return 0, err
	}
	if written != len(colored) {
		return 0, io.ErrShortWrite
	}
	return len(data), nil
}

var levelColors = []struct {
	plain   []byte
	colored []byte
}{
	{plain: []byte("level=DEBUG"), colored: []byte("level=\x1b[36mDEBUG\x1b[0m")},
	{plain: []byte("level=INFO"), colored: []byte("level=\x1b[32mINFO\x1b[0m")},
	{plain: []byte("level=WARN"), colored: []byte("level=\x1b[33mWARN\x1b[0m")},
	{plain: []byte("level=ERROR"), colored: []byte("level=\x1b[31mERROR\x1b[0m")},
}
