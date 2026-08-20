package logger

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Ensure MultiHandler.Enabled respects the level.
func TestMultiHandler_Enabled(t *testing.T) {
	mh := &MultiHandler{console: &consoleHandler{w: io.Discard}, level: logLevel}
	logLevel.Set(slog.LevelInfo)
	if !mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Info should be enabled")
	}
	if mh.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Debug should be disabled at Info level")
	}
}
