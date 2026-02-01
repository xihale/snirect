package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	levelNames = []string{
		"DEBUG",
		"INFO",
		"WARN",
		"ERROR",
	}
	levelColors = []string{
		"\033[36m", // DEBUG: Cyan
		"\033[32m", // INFO: Green
		"\033[33m", // WARN: Yellow
		"\033[31m", // ERROR: Red
	}
	resetColor = "\033[0m"
	
	currentLevel Level = LevelDebug
	mu           sync.Mutex
	logFile      *os.File
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func SetLevel(l string) {
	mu.Lock()
	defer mu.Unlock()
	switch strings.ToUpper(l) {
	case "DEBUG":
		currentLevel = LevelDebug
	case "INFO":
		currentLevel = LevelInfo
	case "WARN", "WARNING":
		currentLevel = LevelWarn
	case "ERROR":
		currentLevel = LevelError
	default:
		// Default to DEBUG if unknown
		currentLevel = LevelDebug
	}
}

func SetOutput(path string) error {
	mu.Lock()
	defer mu.Unlock()
	
	if path == "" {
		log.SetOutput(os.Stderr)
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f
	// MultiWriter to write to both stdout and file if desired, or just file?
	// User requirement usually implies seeing output. Let's write to both if file is specified.
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return nil
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

func logMsg(level Level, format string, v ...interface{}) {
	if level < currentLevel {
		return
	}
	
	// Apply color to the level name
	color := levelColors[level]
	prefix := fmt.Sprintf("%s[%s]%s ", color, levelNames[level], resetColor)
	
	log.SetPrefix(prefix)
	// Calldepth 3: logMsg -> Info/Debug -> caller
	log.Output(3, fmt.Sprintf(format, v...))
}

func Debug(format string, v ...interface{}) { logMsg(LevelDebug, format, v...) }
func Info(format string, v ...interface{}) { logMsg(LevelInfo, format, v...) }
func Warn(format string, v ...interface{}) { logMsg(LevelWarn, format, v...) }
func Error(format string, v ...interface{}) { logMsg(LevelError, format, v...) }
func Fatal(format string, v ...interface{}) {
	logMsg(LevelError, format, v...)
	os.Exit(1)
}
