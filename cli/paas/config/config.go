package config

// Config represents a carrier config
type Config struct {
	GiteaNamespace string
	GiteaProtocol  string

	EiriniWorkloadsNamespace string

	KubeConfig       string
	Org              string
	UseHTTPEndpoints *bool
}

// Load loads the Carrier config
func Load() (*Config, error) {
	cfg := &Config{}

	if cfg.GiteaNamespace == "" {
		cfg.GiteaNamespace = "gitea"
	}

	if cfg.GiteaProtocol == "" {
		cfg.GiteaProtocol = "http"
	}

	if cfg.EiriniWorkloadsNamespace == "" {
		cfg.EiriniWorkloadsNamespace = "eirini-workloads"
	}

	if cfg.Org == "" {
		cfg.Org = "workspace"
	}

	return cfg, nil
}

// Save saves the Carrier config
func (c *Config) Save() error {
	return nil
}
