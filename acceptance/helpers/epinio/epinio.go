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
	// Add a repo for Epinio Helm Chart
	out, err := proc.RunW("helm", "repo", "add", "epinio-helm-chart", "https://epinio.github.io/epinio-helm-chart")
	if err != nil {
		return out, err
	}

	// Update helm repos
	out, err = proc.RunW("helm", "repo", "update")
	if err != nil {
		return out, err
	}

	// Install Epinio
	opts := []string{
		"install",
		"epinio-installer",
		"epinio-helm-chart/epinio-installer",
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
