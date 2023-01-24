package upgraderesponder

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/version"
	"github.com/go-logr/logr"
	"github.com/longhorn/upgrade-responder/client"
)

const (
	UpgradeResponderAddress = "https://epinio.version.rancher.io/v1/checkupgrade"
)

func NewChecker(ctx context.Context, logger logr.Logger, upgradeResponderAddress string) (*client.UpgradeChecker, error) {
	logger = logger.WithName("UpgradeChecker")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return nil, err
	}

	return client.NewUpgradeChecker(upgradeResponderAddress, &EpinioUpgradeRequester{
		Logger:              logger,
		EpinioVersion:       version.ChartVersion,
		EpinioServerVersion: version.Version,
		KubernetesPlatform:  cluster.GetPlatform().String(),
		KubernetesVersion:   kubeVersion,
	}), nil
}

type EpinioUpgradeRequester struct {
	Logger              logr.Logger
	EpinioVersion       string
	EpinioServerVersion string
	KubernetesPlatform  string
	KubernetesVersion   string
}

func (e *EpinioUpgradeRequester) GetCurrentVersion() string {
	return e.EpinioVersion
}

func (e *EpinioUpgradeRequester) GetExtraInfo() map[string]string {
	return map[string]string{
		"epinioServerVersion": e.EpinioServerVersion,
		"kubernetesVersion":   e.KubernetesVersion,
		"kubernetesPlatform":  e.KubernetesPlatform,
	}
}

func (e *EpinioUpgradeRequester) ProcessUpgradeResponse(resp *client.CheckUpgradeResponse, err error) {
	e.Logger.Info("processing upgrade response")

	if err != nil {
		e.Logger.Error(err, "error from responder")
		return
	}

	e.Logger.Info("returned response", "versions", resp.Versions)
}
