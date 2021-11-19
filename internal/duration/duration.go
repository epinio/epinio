// Package duration defines the various durations used throughout
// Epinio, as timeouts, and other.
package duration

import (
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	traefikIP           = 2 * time.Minute
	appReady            = 2 * time.Minute
	deployment          = 10 * time.Minute
	namespaceDeletion   = 5 * time.Minute
	serviceSecret       = 5 * time.Minute
	serviceProvision    = 5 * time.Minute
	serviceLoadBalancer = 5 * time.Minute
	podReady            = 5 * time.Minute
	appBuilt            = 10 * time.Minute
	warmupJobReady      = 30 * time.Minute
	certManagerReady    = 5 * time.Minute
	kubedReady          = 5 * time.Minute
	secretCopied        = 5 * time.Minute

	// Fixed. __Not__ affected by the multiplier.
	pollInterval = 3 * time.Second
	userAbort    = 5 * time.Second
	logHistory   = 48 * time.Hour

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

// ToCertManagerReady returns the duration to wait until giving up on
// the cert manager deployment to become ready.
func ToCertManagerReady() time.Duration {
	return Multiplier() * certManagerReady
}

// ToKubedReady returns the duration to wait until giving up on the
// kube demon deployment to become ready.
func ToKubedReady() time.Duration {
	return Multiplier() * kubedReady
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

// ToTraefikIP returns the duration to wait until the Traefik service gets
// a LoadBalancer IP address.
func ToTraefikIP() time.Duration {
	return Multiplier() * traefikIP
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

// ToNamespaceDeletion returns the duration to wait for deletion of namespace
func ToNamespaceDeletion() time.Duration {
	return Multiplier() * namespaceDeletion
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

// ToServiceLoadBalancer
func ToServiceLoadBalancer() time.Duration {
	return Multiplier() * serviceLoadBalancer
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
