package epinio

import (
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
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

	opts := []string{
		"upgrade",
		"--install",
		"-n",
		"epinio",
		"--create-namespace",
		"epinio",
		"helm-charts/chart/epinio",
		"--wait",
	}

	out, err = proc.Run(testenv.Root(), false, "helm", append(opts, args...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Uninstall() (string, error) {
	out, err := proc.RunW("helm", "uninstall", "-n", "epinio", "epinio")
	if err != nil {
		return out, err
	}
	return out, nil
}
