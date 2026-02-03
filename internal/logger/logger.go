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
	defaultLogger *slog.Logger
	programLevel  *slog.LevelVar
	mu            sync.Mutex
	outWriter     io.Writer = os.Stderr
)

func init() {
	programLevel = new(slog.LevelVar)
	programLevel.Set(slog.LevelDebug) // Default to Debug
	setupLogger()
}

// setupLogger initializes the global logger with the current configuration
func setupLogger() {
	opts := &slog.HandlerOptions{
		Level:       programLevel,
		AddSource:   true,
		ReplaceAttr: replaceAttr,
	}
	handler := NewConsoleHandler(outWriter, opts)
	defaultLogger = slog.New(handler)
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		// Custom time format if needed, but ConsoleHandler handles it
		return a
	}
	if a.Key == slog.SourceKey {
		source, ok := a.Value.Any().(*slog.Source)
		if !ok {
			return a
		}
		// Shorten the path to just the file name
		source.File = filepath.Base(source.File)
		return a
	}
	return a
}

// SetLevel sets the global log level
func SetLevel(l string) {
	switch strings.ToUpper(l) {
	case "DEBUG":
		programLevel.Set(slog.LevelDebug)
	case "INFO":
		programLevel.Set(slog.LevelInfo)
	case "WARN", "WARNING":
		programLevel.Set(slog.LevelWarn)
	case "ERROR":
		programLevel.Set(slog.LevelError)
	default:
		programLevel.Set(slog.LevelDebug)
	}
}

// SetOutput sets the log output file
func SetOutput(path string) error {
	mu.Lock()
	defer mu.Unlock()

	if path == "" {
		outWriter = os.Stderr
	} else {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		// Write to both stdout and file
		outWriter = io.MultiWriter(os.Stderr, f)
	}
	setupLogger()
	return nil
}

// Legacy API wrappers

func Debug(format string, v ...interface{}) {
	log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, v...))
}

func Info(format string, v ...interface{}) {
	log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, v...))
}

func Warn(format string, v ...interface{}) {
	log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, v...))
}

func Error(format string, v ...interface{}) {
	log(context.Background(), slog.LevelError, fmt.Sprintf(format, v...))
}

func Fatal(format string, v ...interface{}) {
	log(context.Background(), slog.LevelError, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// log acts as a helper to capture the correct caller depth
func log(ctx context.Context, level slog.Level, msg string) {
	if !defaultLogger.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	// Skip 3 frames: runtime.Callers, log, Wrapper(Debug/Info/etc)
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	_ = defaultLogger.Handler().Handle(ctx, r)
}

// ConsoleHandler is a custom slog.Handler for pretty terminal output
type ConsoleHandler struct {
	w    io.Writer
	opts *slog.HandlerOptions
	mu   sync.Mutex
}

func NewConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *ConsoleHandler {
	return &ConsoleHandler{w: w, opts: opts}
}

func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.opts.Level.Level() <= level
}

func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timeStr := r.Time.Format("2006/01/02 15:04:05")
	levelStr := r.Level.String()

	// Colors
	var color string
	switch r.Level {
	case slog.LevelDebug:
		color = "\033[36m" // Cyan
	case slog.LevelInfo:
		color = "\033[32m" // Green
	case slog.LevelWarn:
		color = "\033[33m" // Yellow
	case slog.LevelError:
		color = "\033[31m" // Red
	default:
		color = "\033[0m"
	}
	reset := "\033[0m"

	// Source
	sourceStr := ""
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			sourceStr = fmt.Sprintf(" %s:%d:", filepath.Base(f.File), f.Line)
		}
	}

	// Format: time [LEVEL] file:line: message key=value...
	fmt.Fprintf(h.w, "%s %s[%s]%s%s %s", timeStr, color, levelStr, reset, sourceStr, r.Message)

	// Print attributes
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
		return true
	})

	fmt.Fprintln(h.w)
	return nil
}

func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Not fully implemented for this simple wrapper, normally would clone
	return h
}

func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	return h
}
