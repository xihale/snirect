package core

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/xihale/snirect/logger"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	levelNames = map[string]LogLevel{
		"debug": LevelDebug,
		"info":  LevelInfo,
		"warn":  LevelWarn,
		"error": LevelError,
	}
	// currentLogLevel is read from the log pump below and from the slog sink
	// (log_bridge.go) while SetLogLevel writes it, so every access goes through
	// logMu — it used to be written under cbMutex but read unlocked, a plain
	// data race (audit L3).
	logMu           sync.RWMutex
	currentLogLevel = LevelInfo
	lastCb          EngineCallbacks
	cbMutex         sync.RWMutex
	logChan         = make(chan string, 100)

	// Overflow accounting for logChan: instead of dropping silently, count
	// the drops and warn (rate-limited) so a wedged UI consumer is visible
	// in the log it is wedging (audit L3).
	dropMu         sync.Mutex
	droppedCount   int
	lastDropReport time.Time
)

func init() {
	go func() {
		for msg := range logChan {
			cbMutex.RLock()
			cb := lastCb
			cbMutex.RUnlock()
			if cb != nil {
				func() {
					defer func() { recover() }()
					cb.OnStatusChanged(msg)
				}()
			}
		}
	}()
}

// SetLogLevel sets both the mobile log level and snirect-core's. Core's
// MultiHandler pre-filters at its own LevelVar (default INFO), so without the
// logger.SetLevel mirror a "debug" selection here still meant core's DEBUG
// records were dropped before they ever reached the mobile sink (audit L3).
func SetLogLevel(levelStr string) {
	level, ok := levelNames[levelStr]
	logMu.Lock()
	if ok {
		currentLogLevel = level
	}
	logMu.Unlock()
	if ok {
		logger.SetLevel(levelStr)
	}
}

func levelEnabled(level LogLevel) bool {
	logMu.RLock()
	defer logMu.RUnlock()
	return level >= currentLogLevel
}

func logf(level LogLevel, format string, args ...interface{}) {
	if !levelEnabled(level) {
		return
	}

	prefix := ""
	switch level {
	case LevelDebug:
		prefix = "[DEBUG] "
	case LevelInfo:
		prefix = "[INFO] "
	case LevelWarn:
		prefix = "[WARN] "
	case LevelError:
		prefix = "[ERROR] "
	}

	msg := prefix + fmt.Sprintf(format, args...)
	log.Println(msg)

	enqueueLog(msg)
}

// enqueueLog hands a formatted line to the pump, counting and reporting drops
// when the channel is full. The overflow warning bypasses logf on purpose:
// recursing through it could re-enter the overflow path while the queue is
// still full.
func enqueueLog(msg string) {
	select {
	case logChan <- msg:
		return
	default:
	}

	dropMu.Lock()
	droppedCount++
	report := false
	if time.Since(lastDropReport) >= 10*time.Second {
		report = true
		lastDropReport = time.Now()
	}
	n := droppedCount
	if report {
		droppedCount = 0
	}
	dropMu.Unlock()

	if report {
		warn := "[WARN] mobile log queue overflow: dropped " + strconv.Itoa(n) + " message(s)"
		log.Println(warn)
		select {
		case logChan <- warn:
		default:
		}
	}
}

func LogDebug(format string, args ...interface{}) { logf(LevelDebug, format, args...) }
func LogInfo(format string, args ...interface{})  { logf(LevelInfo, format, args...) }
func LogWarn(format string, args ...interface{})  { logf(LevelWarn, format, args...) }
func LogError(format string, args ...interface{}) { logf(LevelError, format, args...) }
