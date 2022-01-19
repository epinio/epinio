package epinio

import (
	"github.com/epinio/epinio/acceptance/helpers/proc"
)

type Epinio struct {
	EpinioBinaryPath string
}

func NewEpinioHelper(epinioBinaryPath string) Epinio {
	return Epinio{
		EpinioBinaryPath: epinioBinaryPath,
	}
}

func (e *Epinio) Run(cmd string, args ...string) (string, error) {
	out, err := proc.RunW(e.EpinioBinaryPath, append([]string{cmd}, args...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Install(args ...string) (string, error) {
	// Update helm repos -- Assumes existence of helm repository providing the helm charts
	out, err := proc.RunW("helm", "repo", "update")
	if err != nil {
		return out, err
	}

	// Install Epinio
	opts := []string{
		"install",
		"epinio-installer",
		"epinio-charts/epinio-installer",
	}
	out, err = proc.RunW("helm", append(opts, args...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Uninstall() (string, error) {
	out, err := proc.RunW("helm", "uninstall", "epinio-installer")
	if err != nil {
		return out, err
	}
	return out, nil
}
