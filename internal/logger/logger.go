package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	currentLogger *slog.Logger
	logLevel      *slog.LevelVar
	mu            sync.RWMutex
	disableColor  bool
)

func init() {
	logLevel = &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	updateLogger("")
}

func SetColorEnabled(enabled bool) {
	disableColor = !enabled
	// Re-initialize with new color setting if needed
	// For simplicity, updateLogger will be called by the app
}

func updateLogger(path string) {
	mu.Lock()
	defer mu.Unlock()

	var writers []io.Writer
	writers = append(writers, os.Stderr)

	if path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writers = append(writers, f)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		}
	}

	multiWriter := io.MultiWriter(writers...)

	opts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(a.Key, a.Value.Time().Format("2006/01/02 15:04:05"))
			}
			return a
		},
	}

	handler := slog.NewTextHandler(multiWriter, opts)
	currentLogger = slog.New(handler)
}

func SetLevel(l string) {
	switch strings.ToUpper(l) {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "INFO":
		logLevel.Set(slog.LevelInfo)
	case "WARN", "WARNING":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelDebug)
	}
}

func SetOutput(path string) error {
	updateLogger(path)
	return nil
}

func logf(level slog.Level, format string, v ...interface{}) {
	if !currentLogger.Enabled(context.Background(), level) {
		return
	}
	msg := fmt.Sprintf(format, v...)
	currentLogger.Log(context.Background(), level, msg)
}

func Debug(format string, v ...interface{}) { logf(slog.LevelDebug, format, v...) }
func Info(format string, v ...interface{})  { logf(slog.LevelInfo, format, v...) }
func Warn(format string, v ...interface{})  { logf(slog.LevelWarn, format, v...) }
func Error(format string, v ...interface{}) { logf(slog.LevelError, format, v...) }
func Fatal(format string, v ...interface{}) {
	logf(slog.LevelError, format, v...)
	os.Exit(1)
}
