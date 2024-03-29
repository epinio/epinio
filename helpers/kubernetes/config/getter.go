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
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Getter is the interface that wraps the Get method that returns the Kubernetes configuration used
// to communicate with it using its API.
type Getter interface {
	Get(configPath string) (*rest.Config, error)
}

// NewGetter constructs a default getter that satisfies the Getter interface.
func NewGetter() Getter {
	return &getter{
		inClusterConfig:          rest.InClusterConfig,
		stat:                     os.Stat,
		restConfigFromKubeConfig: clientcmd.NewNonInteractiveDeferredLoadingClientConfig,
		lookupEnv:                os.LookupEnv,
		currentUser:              user.Current,
		defaultRESTConfig:        clientcmd.DefaultClientConfig.ClientConfig,
	}
}

type getter struct {
	inClusterConfig          func() (*rest.Config, error)
	stat                     func(name string) (os.FileInfo, error)
	restConfigFromKubeConfig func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig
	lookupEnv                func(key string) (string, bool)
	currentUser              func() (*user.User, error)
	defaultRESTConfig        func() (*rest.Config, error)
}

func (g *getter) Get(configPath string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if configPath == "" {
		c, err := g.restConfigFromKubeConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, &getConfigError{err}
		}
		return c, nil
	}

	// Default loading rules are replaced by configPath provided
	// by the epinio user.
	if len(configPath) > 0 {
		paths := filepath.SplitList(configPath)
		if len(paths) == 1 {
			loadingRules.ExplicitPath = paths[0]
		} else {
			loadingRules.Precedence = paths
		}
	}
	c, err := g.restConfigFromKubeConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, &getConfigError{err}
	}

	return c, nil
}

type getConfigError struct {
	err error
}

func (e *getConfigError) Error() string {
	return fmt.Sprintf("failed to get kube config: %v", e.err)
}
