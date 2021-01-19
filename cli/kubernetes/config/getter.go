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
	if configPath == "" {
		// If no explicit location, try the in-cluster config.
		_, okHost := g.lookupEnv("KUBERNETES_SERVICE_HOST")
		_, okPort := g.lookupEnv("KUBERNETES_SERVICE_PORT")
		if okHost && okPort {
			c, err := g.inClusterConfig()
			if err == nil {
				return c, nil
			} else if !os.IsNotExist(err) {
				return nil, &getConfigError{err}
			}
		}

		// If no in-cluster config, set the config path to the user's ~/.kube directory.
		usr, err := g.currentUser()
		if err != nil {
			return nil, &getConfigError{err}
		}

		homeFile := filepath.Join(usr.HomeDir, ".kube", "config")
		_, err = g.stat(homeFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, &getConfigError{err}
			}

			// If neither the custom config path, nor the user's ~/.kube directory config path exist, use a
			// default config.
			c, err := g.defaultRESTConfig()
			if err != nil {
				return nil, &getConfigError{err}
			}

			return c, nil
		}

		configPath = homeFile
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
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
