//+build wireinject

package paas

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"
	"github.com/suse/carrier/cli/kubernetes"
	kubeconfig "github.com/suse/carrier/cli/kubernetes/config"
	"github.com/suse/carrier/cli/paas/config"
	"github.com/suse/carrier/cli/paas/eirini"
	"github.com/suse/carrier/cli/paas/gitea"
	"github.com/suse/carrier/cli/paas/ui"
)

// NewCarrierClient creates the Carrier Client
func NewCarrierClient(flags *pflag.FlagSet, configOverrides func(*config.Config)) (*CarrierClient, func(), error) {
	wire.Build(
		wire.Struct(new(CarrierClient), "*"),
		config.Load,
		ui.NewUI,
		gitea.NewGiteaClient,
		gitea.NewResolver,
		kubernetes.NewClusterFromClient,
		kubeconfig.KubeConfig,
		eirini.NewEiriniKubeClient,
	)

	return &CarrierClient{}, func() {}, nil
}

// NewInstallClient creates the Carrier Client for installation
func NewInstallClient(flags *pflag.FlagSet, configOverrides func(*config.Config)) (*InstallClient, func(), error) {
	wire.Build(
		wire.Struct(new(InstallClient), "*"),
		config.Load,
		ui.NewUI,
		kubernetes.NewClusterFromClient,
		kubeconfig.KubeConfig,
	)

	return &InstallClient{}, func() {}, nil
}
