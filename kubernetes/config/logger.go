package config

import (
	"log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// TraceLevel returns the trace-level argument
func TraceLevel() int {
	return viper.GetInt("trace-level")
}

// LoggerFlags adds to viper flags
func LoggerFlags(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.IntP("trace-level", "", 0, "Only print trace messages at or above this level (0 to 2, default 0)")
	viper.BindPFlag("trace-level", pf.Lookup("trace-level"))
	argToEnv["trace-level"] = "TRACE_LEVEL"
}

// New creates a new logger with our setup
func NewClientLogger() logr.Logger {
	return NewLogger().WithName("EpinioClient")
}

// New creates a new logger with our setup
func NewInstallClientLogger() logr.Logger {
	return NewLogger().WithName("InstallClient")
}

// New creates a new logger with our setup
func NewLogger() logr.Logger {
	stdr.SetVerbosity(TraceLevel())
	return stdr.New(log.New(os.Stderr, "", log.LstdFlags)).V(1) // NOTE: Increment of level, not absolute.
}
