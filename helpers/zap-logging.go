package helpers

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

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

func InitLogger() error {
	// Define flags
	//pflag.String("log-level", "info", "debug, info, warn, error, fatal")
	//pflag.Bool("color", false, "enable colored log levels")
	//pflag.Parse()

	// Bind flags into Viper
	//viper.BindPFlags(pflag.CommandLine)

	// Environment overrides
	//viper.SetEnvPrefix("APP")
	//viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	//viper.AutomaticEnv()

	// Parse log level
	logLevel := viper.GetString("log-level")
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
