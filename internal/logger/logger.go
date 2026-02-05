package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	currentLogger *slog.Logger
	logLevel      *slog.LevelVar
	mu            sync.RWMutex
	disableColor  bool
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func init() {
	logLevel = &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	updateLogger("")
}

func SetColorEnabled(enabled bool) {
	mu.Lock()
	disableColor = !enabled
	mu.Unlock()
	// Re-initialize logger to apply color settings if console writer is used
	// Note: We don't have the path here, assuming it's kept or managed by updateLogger caller usually.
	// But updateLogger requires path. For simplicty, let's just update console writer logic next time updateLogger is called
	// OR we can store the last path. Let's store the last path.
}

var lastLogPath string

func updateLogger(path string) {
	mu.Lock()
	defer mu.Unlock()
	lastLogPath = path

	var writers []io.Writer

	// Console Writer with custom formatting
	consoleWriter := &consoleHandler{
		w:            os.Stderr,
		disableColor: disableColor,
	}
	writers = append(writers, consoleWriter)

	// Ensure directory exists if path is provided
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writers = append(writers, f)
		} else {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		}
	}

	handler := &MultiHandler{
		console: consoleWriter,
		file:    nil,
		level:   logLevel,
	}

	// If a file writer was added, it's the second writer
	if len(writers) > 1 {
		handler.file = slog.NewTextHandler(writers[1], &slog.HandlerOptions{
			Level: logLevel,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.String(a.Key, a.Value.Time().Format("2006/01/02 15:04:05"))
				}
				return a
			},
		})
	}

	currentLogger = slog.New(handler)

	// Log the initial path info only if file logging is enabled
	if len(writers) > 1 {
		absPath, _ := filepath.Abs(path)
		currentLogger.Info(fmt.Sprintf("Logging to file: %s", absPath))
	}
}

// MultiHandler dispatches to console (custom) and file (standard text)
type MultiHandler struct {
	console *consoleHandler
	file    slog.Handler
	level   slog.Leveler
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Write to console
	h.console.Handle(ctx, r)

	// Write to file if configured
	if h.file != nil {
		h.file.Handle(ctx, r)
	}
	return nil
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := &MultiHandler{
		console: h.console, // Console handler doesn't support attrs yet in this simple version
		level:   h.level,
	}
	if h.file != nil {
		newH.file = h.file.WithAttrs(attrs)
	}
	return newH
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newH := &MultiHandler{
		console: h.console,
		level:   h.level,
	}
	if h.file != nil {
		newH.file = h.file.WithGroup(name)
	}
	return newH
}

// consoleHandler formats logs for human readability
type consoleHandler struct {
	w            io.Writer
	disableColor bool
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	msg := r.Message
	timeStr := r.Time.Format("15:04:05")

	var levelColor, reset string
	if !h.disableColor {
		reset = colorReset
		switch r.Level {
		case slog.LevelDebug:
			levelColor = colorGray
		case slog.LevelInfo:
			levelColor = colorGreen
		case slog.LevelWarn:
			levelColor = colorYellow
		case slog.LevelError:
			levelColor = colorRed
		}
	}

	// Format: [15:04:05] [INFO] message key=value...
	_, err := fmt.Fprintf(h.w, "%s[%s] [%s%s%s] %s%s\n",
		colorGray, timeStr,
		levelColor, level, colorGray,
		reset, msg)
	return err
}

// Write implements io.Writer to support standard logger compatibility if needed
func (h *consoleHandler) Write(p []byte) (n int, err error) {
	return h.w.Write(p)
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
	// For slog, we don't format the message here if we want to support structured logging fully,
	// but since the interface mimics printf, we format it.
	msg := fmt.Sprintf(format, v...)

	// Helper to skip caller frames if we were using AddSource (not used here for simplicity)
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:]) // capture caller
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	currentLogger.Handler().Handle(context.Background(), r)
}

func Debug(format string, v ...interface{}) { logf(slog.LevelDebug, format, v...) }
func Info(format string, v ...interface{})  { logf(slog.LevelInfo, format, v...) }
func Warn(format string, v ...interface{})  { logf(slog.LevelWarn, format, v...) }
func Error(format string, v ...interface{}) { logf(slog.LevelError, format, v...) }
func Fatal(format string, v ...interface{}) {
	logf(slog.LevelError, format, v...)
	os.Exit(1)
}
