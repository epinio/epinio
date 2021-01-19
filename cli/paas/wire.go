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

// BuildApp creates the Carrier Client
func BuildApp(flags *pflag.FlagSet, configOverrides func(*config.Config)) (*CarrierClient, func(), error) {
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
