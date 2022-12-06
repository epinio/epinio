package upgraderesponder

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/version"
	"github.com/go-logr/logr"
	"github.com/longhorn/upgrade-responder/client"
)

func NewChecker(ctx context.Context, logger logr.Logger, host string) (*client.UpgradeChecker, error) {
	logger = logger.WithName("UpgradeChecker")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("%s/v1/checkupgrade", host)

	return client.NewUpgradeChecker(address, &EpinioUpgradeRequester{
		Logger:             logger,
		EpinioVersion:      version.Version,
		KubernetesPlatform: cluster.GetPlatform().String(),
		KubernetesVersion:  kubeVersion,
	}), nil
}

type EpinioUpgradeRequester struct {
	Logger             logr.Logger
	EpinioVersion      string
	KubernetesPlatform string
	KubernetesVersion  string
}

func (e *EpinioUpgradeRequester) GetCurrentVersion() string {
	return e.EpinioVersion
}

func (e *EpinioUpgradeRequester) GetExtraInfo() map[string]string {
	return map[string]string{
		"kubernetesVersion":  e.KubernetesVersion,
		"kubernetesPlatform": e.KubernetesPlatform,
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
