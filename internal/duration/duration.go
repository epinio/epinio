// Package duration defines the various durations used throughout
// Epinio, as timeouts, and other.
package duration

import (
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	deployment          = 10 * time.Minute
	namespaceDeletion   = 5 * time.Minute
	configurationSecret = 5 * time.Minute
	appBuilt            = 10 * time.Minute
	secretCopied        = 5 * time.Minute

	// Fixed. __Not__ affected by the multiplier.
	userAbort  = 5 * time.Second
	logHistory = 48 * time.Hour

	// Fixed. Standard number of attempts to retry various operations.
	RetryMax = 10
)

// Flags adds to viper flags
func Flags(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.IntP("timeout-multiplier", "", 1, "Multiply timeouts by this factor")
	viper.BindPFlag("timeout-multiplier", pf.Lookup("timeout-multiplier"))
	argToEnv["timeout-multiplier"] = "EPINIO_TIMEOUT_MULTIPLIER"
}

// Multiplier returns the currently active timeout multiplier value
func Multiplier() time.Duration {
	return time.Duration(viper.GetInt("timeout-multiplier"))
}

// ToSecretCopied returns the duration to wait until giving up on a
// secret getting copied to complete.
func ToSecretCopied() time.Duration {
	return Multiplier() * secretCopied
}

// ToAppBuilt returns the duration to wait until giving up on the
// application being built
func ToAppBuilt() time.Duration {
	return Multiplier() * appBuilt
}

// ToDeployment returns the duration to wait for parts of a deployment
// to become ready
func ToDeployment() time.Duration {
	return Multiplier() * deployment
}

// ToNamespaceDeletion returns the duration to wait for deletion of namespace
func ToNamespaceDeletion() time.Duration {
	return Multiplier() * namespaceDeletion
}

// ToConfigurationSecret returns the duration to wait for the secret to a
// catalog configuration binding to appear
func ToConfigurationSecret() time.Duration {
	return Multiplier() * configurationSecret
}

//
// The following durations are not affected by the timeout multiplier.
//

// UserAbort returns the duration to wait when the user is given the
// chance to abort an operation
func UserAbort() time.Duration {
	return userAbort
}

// LogHistory returns the duration to reach into the past for tailing logs.
func LogHistory() time.Duration {
	return logHistory
}
