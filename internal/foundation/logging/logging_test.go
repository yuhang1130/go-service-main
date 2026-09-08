package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestTextLoggerColorsLevelsWhenEnabled(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := newLogger(Config{Level: "info", Format: "text"}, &output, true)

	logger.Info("ready")

	if !strings.Contains(output.String(), "level=\x1b[32mINFO\x1b[0m") {
		t.Fatalf("colored level missing from %q", output.String())
	}
}

func TestJSONLoggerNeverAddsTerminalColors(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := newLogger(Config{Level: "info", Format: "json"}, &output, true)

	logger.Info("ready")

	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("JSON log contains terminal color escapes: %q", output.String())
	}
}
