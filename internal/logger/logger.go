package logger

import (
	"os"
	"strings"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sugar *zap.SugaredLogger
	atom  zap.AtomicLevel
)

func init() {
	atom = zap.NewAtomicLevelAt(zap.DebugLevel)

	// Default setup: only console
	core := zapcore.NewCore(
		getConsoleEncoder(true),
		zapcore.Lock(os.Stderr),
		atom,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugar = logger.Sugar()
}

func getConsoleEncoder(color bool) zapcore.Encoder {
	config := zap.NewDevelopmentEncoderConfig()
	if color {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05")
	config.CallerKey = "caller"
	config.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewConsoleEncoder(config)
}

// SetLevel sets the global log level
func SetLevel(l string) {
	switch strings.ToUpper(l) {
	case "DEBUG":
		atom.SetLevel(zap.DebugLevel)
	case "INFO":
		atom.SetLevel(zap.InfoLevel)
	case "WARN", "WARNING":
		atom.SetLevel(zap.WarnLevel)
	case "ERROR":
		atom.SetLevel(zap.ErrorLevel)
	default:
		atom.SetLevel(zap.DebugLevel)
	}
}

// SetOutput sets the log output file with rotation
func SetOutput(path string) error {
	var cores []zapcore.Core

	// 1. Console Core (Always colored)
	cores = append(cores, zapcore.NewCore(
		getConsoleEncoder(true),
		zapcore.Lock(os.Stderr),
		atom,
	))

	// 2. File Core (No colors)
	if path != "" {
		w := zapcore.AddSync(&lumberjack.Logger{
			Filename:   path,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		})

		cores = append(cores, zapcore.NewCore(
			getConsoleEncoder(false), // No colors for file
			w,
			atom,
		))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugar = logger.Sugar()
	return nil
}

func Debug(format string, v ...interface{}) {
	sugar.Debugf(format, v...)
}

func Info(format string, v ...interface{}) {
	sugar.Infof(format, v...)
}

func Warn(format string, v ...interface{}) {
	sugar.Warnf(format, v...)
}

func Error(format string, v ...interface{}) {
	sugar.Errorf(format, v...)
}

func Fatal(format string, v ...interface{}) {
	sugar.Fatalf(format, v...)
}
