// Package duration defines the various durations used throughout
// Epinio, as timeouts, and other.
package duration

import (
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	systemDomain     = 2 * time.Minute
	appReady         = 2 * time.Minute
	deployment       = 5 * time.Minute
	serviceSecret    = 5 * time.Minute
	serviceProvision = 5 * time.Minute
	podReady         = 5 * time.Minute
	appBuilt         = 10 * time.Minute
	warmupJobReady   = 30 * time.Minute

	// Fixed. __Not__ affected by the multiplier.
	pollInterval = 3 * time.Second
	userAbort    = 5 * time.Second
	logHistory   = 48 * time.Hour
)

// Flags adds to viper flags
func Flags(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.IntP("timeout-multiplier", "", 1, "Multiply timeouts by this factor")
	viper.BindPFlag("timeout-multiplier", pf.Lookup("timeout-multiplier"))
	argToEnv["timeout-multiplier"] = "EPINIO_TIMEOUT_MULTIPLIER"
}

// Multiplier returns the timeout-multiplier argument
func Multiplier() time.Duration {
	return time.Duration(viper.GetInt("timeout-multiplier"))
}

// ToAppBuilt returns the duration to wait until giving up on the
// application being built
func ToAppBuilt() time.Duration {
	return Multiplier() * appBuilt
}

// ToPodReady returns the duration to wait until giving up on getting
// a system domain
func ToPodReady() time.Duration {
	return Multiplier() * podReady
}

// ToWarmupJobReady return the duration to wait until the builder image
// warm up job is Complete. The time it takes for that Job to complete depends
// on the network speed of the cluster so be generous here.
func ToWarmupJobReady() time.Duration {
	return Multiplier() * warmupJobReady
}

// ToSystemDomain returns the duration to wait until giving on getting
// the system domain
func ToSystemDomain() time.Duration {
	return Multiplier() * systemDomain
}

// ToAppReady returns the duration to wait until the curl request
// on app url
func ToAppReady() time.Duration {
	return Multiplier() * appReady
}

// ToDeployment returns the duration to wait for parts of a deployment
// to become ready
func ToDeployment() time.Duration {
	return Multiplier() * deployment
}

// ToServiceSecret returns the duration to wait for the secret to a
// catalog service binding to appear
func ToServiceSecret() time.Duration {
	return Multiplier() * serviceSecret
}

// ToServiceProvision returns the duration to wait for a catalog
// service instance to be provisioned
func ToServiceProvision() time.Duration {
	return Multiplier() * serviceProvision
}

//
// The following durations are not affected by the timeout multiplier.
//

// PollInterval returns the duration to use between polls of some kind
// of check.
func PollInterval() time.Duration {
	return pollInterval
}

// UserAbort returns the duration to wait when the user is given the
// chance to abort an operation
func UserAbort() time.Duration {
	return userAbort
}

// LogHistory returns the duration to reach into the past for tailing logs.
func LogHistory() time.Duration {
	return logHistory
}
