package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
)

// KubeConfigFlags adds a kubeconfig flag to the set
func KubeConfigFlags(pf *pflag.FlagSet, argToEnv map[string]string) {
	pf.StringP("kubeconfig", "c", "", "path to a kubeconfig, not required in-cluster")
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	argToEnv["kubeconfig"] = "KUBECONFIG"
}

// KubeConfig uses kubeconfig pkg to return a valid kube config
func KubeConfig() (*rest.Config, error) {
	restConfig, err := NewGetter().Get(viper.GetString("kubeconfig"))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch kubeconfig; ensure kubeconfig is present to continue")
	}
	if err := NewChecker().Check(restConfig); err != nil {
		return nil, errors.Wrap(err, "couldn't check kubeconfig; ensure kubeconfig is correct to continue")
	}
	return restConfig, nil
}
