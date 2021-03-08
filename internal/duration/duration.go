// Package duration defines the various durations used throughout
// Carrier, as timeouts, and other.
package duration

import (
	"time"
)

const (
	multiplier    = 1
	systemDomain  = 2 * time.Minute
	deployment    = 5 * time.Minute
	serviceSecret = 5 * time.Minute
	podReady      = 5 * time.Minute
	appBuilt      = 33 * time.Minute

	// Fixed. __Not__ affected by the multiplier.
	pollInterval = 3 * time.Second
	userAbort    = 5 * time.Second
	logHistory   = 48 * time.Hour
)

// LogHistory returns the duration to reach into the past for tailing logs.
func LogHistory() time.Duration {
	return logHistory
}

// PollInterval returns the duration to use between polls of some kind
// of check.
func PollInterval() time.Duration {
	return pollInterval
}

// ToAppBuilt returns the duration to wait until giving up on the
// application being built
func ToAppBuilt() time.Duration {
	return multiplier * appBuilt
}

// ToPodReady returns the duration to wait until giving up on getting
// a system domain
func ToPodReady() time.Duration {
	return multiplier * podReady
}

// ToSystemDomain returns the duration to wait until giving on getting
// the system domain
func ToSystemDomain() time.Duration {
	return multiplier * systemDomain
}

// ToDeployment returns the duration to wait for parts of a deployment
// to become ready
func ToDeployment() time.Duration {
	return multiplier * deployment
}

// ToServiceSecret returns the duration to wait for the secret to a
// catalog service binding to appear
func ToServiceSecret() time.Duration {
	return multiplier * serviceSecret
}

// UserAbort returns the duration to wait when the user is given the
// chance to abort an operation
func UserAbort() time.Duration {
	return userAbort
}
