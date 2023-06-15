// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	// Initialize common client auth plugins (`init` block of the imported package)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

// KubeConfigFlags adds a kubeconfig flag to the set
func KubeConfigFlags(pf *pflag.FlagSet, argToEnv map[string]string) {
	pf.StringP("kubeconfig", "c", "", "path to a kubeconfig, not required in-cluster")
	err := viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	checkErr(err)
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

	restConfig.QPS = float32(viper.GetFloat64("kube-api-qps"))
	restConfig.Burst = viper.GetInt("kube-api-burst")

	return restConfig, nil
}
