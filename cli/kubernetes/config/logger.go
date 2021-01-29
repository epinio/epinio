package config

import (
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
)

// Verbosity returns the verbosity argument
func Verbosity() int {
	return viper.GetInt("verbosity")
}

// LoggerFlags adds to viper flags
func LoggerFlags(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.IntP("verbosity", "", 0, "Only print log messages at or above this level")
	viper.BindPFlag("verbosity", pf.Lookup("verbosity"))
	argToEnv["verbosity"] = "VERBOSITY"
}

// New creates a new logger with our setup
func NewClientLogger() logr.Logger {
	return NewLogger().WithName("CarrierClient")
}

// New creates a new logger with our setup
func NewInstallClientLogger() logr.Logger {
	return NewLogger().WithName("InstallClient")
}

// New creates a new logger with our setup
func NewLogger() logr.Logger {
	stdr.SetVerbosity(Verbosity())
	return stdr.New(nil).V(1) // NOTE: Increment of level, not absolute.
}
