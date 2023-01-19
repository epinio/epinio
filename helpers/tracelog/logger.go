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
	"log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
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

// TraceOutput returns the trace-output argument
func TraceOutput() string {
	return viper.GetString("trace-output")
}

// LoggerFlags adds to viper flags
func LoggerFlags(pf *flag.FlagSet, argToEnv map[string]string) {
	// trace-level 0 prints nothing, well technically it would print NewLogger().V(-1)
	pf.IntP("trace-level", "", 0, "Only print trace messages at or above this level (0 to 255, default 0, print nothing)")
	err := viper.BindPFlag("trace-level", pf.Lookup("trace-level"))
	if err != nil {
		log.Fatal(err)
	}
	argToEnv["trace-level"] = "TRACE_LEVEL"
}

// NewLogger returns a logger based on the trace-output configuration
func NewLogger() logr.Logger {
	if TraceOutput() == "json" {
		return NewZapLogger()
	}
	return NewStdrLogger()
}

// NewStdrLogger returns a stdr logger
func NewStdrLogger() logr.Logger {
	return stdr.New(log.New(os.Stderr, "", log.LstdFlags)).V(1) // NOTE: Increment of level, not absolute.
}

// NewZapLogger creates a new zap logger with our setup. It only prints messages below
// Zap uses semantically named levels for logging (DebugLevel, InfoLevel, WarningLevel, ...).
// Logr uses arbitrary numeric levels. By default logr's V(0) is zap's InfoLevel and V(1) is zap's DebugLevel (which is numerically -1).
// Zap does not have named levels that are more verbose than DebugLevel, but it's possible to fake it.
//
// https://github.com/go-logr/zapr#increasing-verbosity
func NewZapLogger() logr.Logger {
	var logger logr.Logger

	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(zapcore.Level(TraceLevel() * -1))

	z, err := zc.Build()
	if err != nil {
		logger = NewStdrLogger()
		logger.Error(err, "error building zap config, using stdr logger as fallback")
	} else {
		logger = zapr.NewLogger(z)
	}

	return logger
}
