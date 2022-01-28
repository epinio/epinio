package epinio

import (
	"strings"

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

	// Get chartmuseum service IP for local chartmuseum
	out, err = proc.RunW("kubectl", "get", "svc",
		"--selector", "app.kubernetes.io/name=chartmuseum",
		"-o", "jsonpath='{.items[0].spec.clusterIP}'")
	if err != nil {
		return out, err
	}
	internal_ip := strings.ReplaceAll(out, "'", "")

	// Install Epinio
	opts := []string{
		"upgrade",
		"--install",
		"--set", "containerRegistryChart=http://" + internal_ip + ":8080/charts/container-registry-0.1.0.tgz",
		"--set", "epinioChart=http://" + internal_ip + ":8080/charts/epinio-0.1.0.tgz",
		"epinio-installer",
		"helm-charts/chart/epinio-installer",
	}

	out, err = proc.Run(testenv.Root(), false, "helm", append(opts, args...)...)
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
