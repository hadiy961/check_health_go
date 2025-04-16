package logger

import (
	"CheckHealthDO/internal/pkg/config"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log is the global logger instance
	Log *zap.Logger
	// Sugar is the global sugared logger instance
	Sugar *zap.SugaredLogger
)

// Init initializes the global logger with configuration
func Init(cfg *config.Config) error {
	if !cfg.Logs.Enabled {
		// Create a no-op logger if logging is disabled
		Log = zap.NewNop()
		Sugar = Log.Sugar()
		return nil
	}

	// Configure level
	level, err := getLogLevel(cfg.Logs.Level)
	if err != nil {
		return err
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Logs.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create core
	var core zapcore.Core

	// Create writers
	var writers []zapcore.WriteSyncer

	// Add file writer if file path is provided
	if cfg.Logs.FilePath != "" {
		if err := os.MkdirAll(cfg.Logs.FilePath, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile := filepath.Join(cfg.Logs.FilePath, fmt.Sprintf("%s.log", cfg.AppName))

		// Configure log rotation using lumberjack
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    100, // megabytes
			MaxBackups: 5,
			MaxAge:     30, // days
			Compress:   true,
		})

		writers = append(writers, fileWriter)
	}

	// Add stdout writer if enabled
	if cfg.Logs.Stdout {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	// Create writer for all outputs
	multiWriter := zapcore.NewMultiWriteSyncer(writers...)

	// Create core with all writers
	core = zapcore.NewCore(encoder, multiWriter, zap.NewAtomicLevelAt(level))

	// Create logger
	// Add CallerSkip(1) to skip the wrapper functions and show the actual caller location
	Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))

	// Add app name as a field
	Log = Log.With(zap.String("app", cfg.AppName))

	// Create sugared logger
	Sugar = Log.Sugar()

	// Log initialization
	Sugar.Infof("Logger initialized with level: %s, format: %s", cfg.Logs.Level, cfg.Logs.Format)

	return nil
}

// Sync flushes any buffered log entries
func Sync() error {
	if Log != nil {
		return Log.Sync()
	}
	return nil
}

// getLogLevel converts a string level to a zapcore.Level
func getLogLevel(levelStr string) (zapcore.Level, error) {
	switch levelStr {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "dpanic":
		return zapcore.DPanicLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", levelStr)
	}
}

// Debug logs a message at DebugLevel with structured fields
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Info logs a message at InfoLevel with structured fields
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Warn logs a message at WarnLevel with structured fields
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel with structured fields
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a message at FatalLevel with structured fields,
// then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// Debugf logs a formatted message at DebugLevel
func Debugf(format string, args ...interface{}) {
	Sugar.Debugf(format, args...)
}

// Infof logs a formatted message at InfoLevel
func Infof(format string, args ...interface{}) {
	Sugar.Infof(format, args...)
}

// Warnf logs a formatted message at WarnLevel
func Warnf(format string, args ...interface{}) {
	Sugar.Warnf(format, args...)
}

// Errorf logs a formatted message at ErrorLevel
func Errorf(format string, args ...interface{}) {
	Sugar.Errorf(format, args...)
}

// Fatalf logs a formatted message at FatalLevel,
// then calls os.Exit(1).
func Fatalf(format string, args ...interface{}) {
	Sugar.Fatalf(format, args...)
}

// With creates a child logger with the field added to it
func With(fields ...zap.Field) *zap.Logger {
	return Log.With(fields...)
}

// WithFields creates a sugared child logger with the fields added to it
func WithFields(args ...interface{}) *zap.SugaredLogger {
	return Sugar.With(args...)
}

// Field creation helpers
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func Float64(key string, value float64) zap.Field {
	return zap.Float64(key, value)
}

func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}
