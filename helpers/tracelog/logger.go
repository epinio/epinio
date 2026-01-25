// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tracelog provides a logger for debugging and tracing
// This logger will not print anything, unless TRACE_LEVEL is at least 1
package tracelog

import (
	"fmt"

	"github.com/epinio/epinio/helpers"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TraceLevel returns the trace-level argument
func TraceLevel() int {
	return viper.GetInt("trace-level")
}

// TraceFile returns the trace-file argument, i.e. the path to the log file to use, if any.
func TraceFile() string {
	return viper.GetString("trace-file")
}

// TraceOutput returns the trace-output argument, i.e. the chosen trace format
func TraceOutput() string {
	return viper.GetString("trace-output")
}

// LoggerFlags adds to viper flags
func LoggerFlags(pf *flag.FlagSet, argToEnv map[string]string) {
	// trace-level 0 prints nothing, well technically it would print NewLogger().V(-1)
	pf.IntP("trace-level", "", 0, "Only print trace messages at or above this level (0 to 255, default 0, print nothing)")
	err := viper.BindPFlag("trace-level", pf.Lookup("trace-level"))
	if err != nil {
		// Use panic for early initialization errors before helpers.Logger is available
		panic(fmt.Sprintf("failed to bind trace-level flag: %v", err))
	}
	argToEnv["trace-level"] = "TRACE_LEVEL"

	pf.StringP("trace-file", "", "", "Print trace messages to the specified file")
	err = viper.BindPFlag("trace-file", pf.Lookup("trace-file"))
	if err != nil {
		panic(fmt.Sprintf("failed to bind trace-file flag: %v", err))
	}
	argToEnv["trace-file"] = "TRACE_FILE"

	pf.String("trace-output", "text", "Sets trace output format [text,json]")
	err = viper.BindPFlag("trace-output", pf.Lookup("trace-output"))
	if err != nil {
		panic(fmt.Sprintf("failed to bind trace-output flag: %v", err))
	}
	argToEnv["trace-output"] = "TRACE_OUTPUT"
}

// NewLogger returns a logger based on the trace-output/trace-file configuration.
// It prefers the centralized helpers.Logger when available, falling back to
// the legacy tracelog configuration for backward compatibility.
func NewLogger() logr.Logger {
	// Use centralized Zap logger if available
	if helpers.Logger != nil {
		return helpers.LoggerToLogr()
	}

	return NewZapLogger()
}

// NewStdrLogger returns a stdr logger
func NewStdrLogger() logr.Logger {
	// Zap-only logging; keep the function for compatibility.
	return NewZapLogger()
}

// NewZapLogger creates a new zap logger with our setup. It only prints messages below
// Zap uses semantically named levels for logging (DebugLevel, InfoLevel, WarningLevel, ...).
// Logr uses arbitrary numeric levels. By default logr's V(0) is zap's InfoLevel and V(1) is zap's DebugLevel (which is numerically -1).
// Zap does not have named levels that are more verbose than DebugLevel, but it's possible to fake it.
//
// https://github.com/go-logr/zapr#increasing-verbosity

func NewZapLogger() logr.Logger {
	// Use centralized Zap logger if available
	if helpers.Logger != nil {
		return helpers.LoggerToLogr()
	}

	// Create a zap logger only if centralized logger is not available
	var cfg zap.Config

	level := TraceLevel()
	// Prevent wrap around in zap internals
	if level > 128 {
		level = 128
	}

	if TraceOutput() == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.Encoding = "console"
	}

	cfg.Level = zap.NewAtomicLevelAt(zapcore.Level(level * -1)) //nolint:gosec

	traceFilePath := TraceFile()
	if traceFilePath != "" {
		//TODO ensure we arent logging sensitive data here
		cfg.OutputPaths = []string{traceFilePath}
	}

	z, err := cfg.Build()
	if err != nil {
		return logr.Discard()
	}

	return zapr.NewLogger(z)
}
