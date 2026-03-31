package helpers

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

// LoggerToLogr converts the centralized Zap logger to a logr.Logger interface.
// This allows code that uses logr.Logger to use the centralized Zap logger.
// This is a compatibility bridge for interoperability with libraries and Kubernetes
// ecosystem code that use the logr.Logger interface (e.g., upgraderesponder, tailer).
func LoggerToLogr() logr.Logger {
	if Logger == nil {
		// Return a no-op logger if Logger hasn't been initialized yet
		return logr.Discard()
	}
	// Convert SugaredLogger back to Logger, then wrap with zapr
	return zapr.NewLogger(Logger.Desugar())
}

// SugaredLoggerToLogr converts a zap.SugaredLogger to a logr.Logger interface.
// This is a compatibility bridge for interoperability with libraries and Kubernetes
// ecosystem code that use the logr.Logger interface (e.g., tailer, watcher).
func SugaredLoggerToLogr(logger *zap.SugaredLogger) logr.Logger {
	if logger == nil {
		return logr.Discard()
	}
	return zapr.NewLogger(logger.Desugar())
}

// Colored level encoder reused from your original file
func coloredLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var lvl string
	switch l {
	case zapcore.DebugLevel:
		lvl = color.BlueString("DEBUG")
	case zapcore.InfoLevel:
		lvl = color.GreenString("INFO")
	case zapcore.WarnLevel:
		lvl = color.YellowString("WARN")
	case zapcore.ErrorLevel:
		lvl = color.RedString("ERROR")
	case zapcore.DPanicLevel, zapcore.PanicLevel:
		lvl = color.HiRedString("PANIC")
	case zapcore.FatalLevel:
		lvl = color.MagentaString("FATAL")
	default:
		lvl = l.String()
	}
	enc.AppendString(lvl)
}

func InitLogger(logLevel string) error {
	// Parse log level
	if logLevel == "" {
		logLevel = "info" // Default to info level if not specified
	}
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(logLevel)); err != nil {
		return fmt.Errorf("invalid log level '%s'", logLevel)
	}

	// Dev config is perfect for now
	cfg := zap.NewDevelopmentConfig()
	cfg.Encoding = "console"

	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.MessageKey = "msg"

	// Define the console line format
	cfg.EncoderConfig.ConsoleSeparator = " | "

	cfg.Level = zap.NewAtomicLevelAt(lvl)

	// Optional level coloring
	if viper.GetBool("no-colors") {
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	} else {
		cfg.EncoderConfig.EncodeLevel = coloredLevelEncoder
	}

	// Build logger
	z, err := cfg.Build()
	if err != nil {
		return err
	}

	Logger = z.Sugar()
	return nil
}

func init() {
	if Logger == nil {
		if err := InitLogger("error"); err != nil {
			// Fallback if logger initialization failed - use standard log
			_ = errors.Wrap(err, "failed to initialize logger")
		}
	}
}
