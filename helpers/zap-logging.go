package helpers

import (
	"context"
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

// consoleCore is the stdout/stderr zap core built by InitLogger. Kept so
// TeeCore can combine it with an extra core (e.g. otelzap) without losing
// the original console encoding settings.
var consoleCore zapcore.Core

// LoggerToLogr converts the centralized Zap logger to a logr.Logger interface.
// This allows code that uses logr.Logger to use the centralized Zap logger.
// This is a compatibility bridge for interoperability with libraries and Kubernetes
// ecosystem code that use the logr.Logger interface (e.g., tailer).
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

	consoleCore = z.Core()
	Logger = z.Sugar()
	return nil
}

// TeeCore replaces the global Logger with a tee of the original console
// core and the provided extra core. Used to attach the otelzap bridge so
// logs are both printed and exported via OpenTelemetry. Context fields
// (used by otelzap for correlation) are stripped from the console path so
// they do not clutter standard log output.
func TeeCore(extra zapcore.Core) {
	if consoleCore == nil || extra == nil {
		return
	}
	Logger = zap.New(
		zapcore.NewTee(stripContextCore{Core: consoleCore}, extra),
		zap.AddCaller(),
	).Sugar()
}

// stripContextCore drops fields whose value is a context.Context before
// delegating to the wrapped core. otelzap uses those fields for emit
// correlation and does not want them printed on stdout.
type stripContextCore struct {
	zapcore.Core
}

func (s stripContextCore) With(fields []zapcore.Field) zapcore.Core {
	return stripContextCore{Core: s.Core.With(filterContextFields(fields))}
}

func (s stripContextCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(ent.Level) {
		return ce.AddCore(ent, s)
	}
	return ce
}

func (s stripContextCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	return s.Core.Write(ent, filterContextFields(fields))
}

func filterContextFields(fields []zapcore.Field) []zapcore.Field {
	out := make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		if _, ok := f.Interface.(context.Context); ok {
			continue
		}
		out = append(out, f)
	}
	return out
}

func init() {
	if Logger == nil {
		if err := InitLogger("error"); err != nil {
			// Fallback if logger initialization failed - use standard log
			_ = errors.Wrap(err, "failed to initialize logger")
		}
	}
}
