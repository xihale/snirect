package logger

import (
	"os"
	"strings"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sugar        *zap.SugaredLogger
	atom         zap.AtomicLevel
	disableColor bool
)

func SetColorEnabled(enabled bool) {
	disableColor = !enabled
}

func init() {
	atom = zap.NewAtomicLevelAt(zap.InfoLevel)
	updateLogger("")
}

func updateLogger(path string) {
	var cores []zapcore.Core

	cores = append(cores, zapcore.NewCore(
		getConsoleEncoder(!disableColor),
		zapcore.Lock(os.Stderr),
		atom,
	))

	if path != "" {
		w := zapcore.AddSync(&lumberjack.Logger{
			Filename:   path,
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		})

		cores = append(cores, zapcore.NewCore(
			getConsoleEncoder(false),
			w,
			atom,
		))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core)
	sugar = logger.Sugar()
}

func getConsoleEncoder(color bool) zapcore.Encoder {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05")
	if color {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	config.CallerKey = "" // Disable caller
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

func SetOutput(path string) error {
	updateLogger(path)
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
