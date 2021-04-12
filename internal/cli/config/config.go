package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var (
	defaultConfigFilePath = os.ExpandEnv("${HOME}/.config/epinio/config.yaml")
)

// Config represents a epinio config
type Config struct {
	GiteaProtocol string `mapstructure:"gitea_protocol"`
	Org           string `mapstructure:"org"`

	v *viper.Viper
}

// DefaultLocation returns the standard location for the configuration file
func DefaultLocation() string {
	return defaultConfigFilePath
}

// Load loads the Epinio config
func Load() (*Config, error) {
	v := viper.New()
	file := location()

	v.SetConfigType("yaml")
	v.SetConfigFile(file)
	v.SetEnvPrefix("EPINIO")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	v.SetDefault("gitea_namespace", "gitea")
	v.SetDefault("gitea_protocol", "http")
	v.SetDefault("epinio_workloads_namespace", "epinio-workloads")
	v.SetDefault("org", "workspace")

	configExists, err := fileExists(file)
	if err != nil {
		return nil, errors.Wrapf(err, "filesystem error")
	}

	if configExists {
		if err := v.ReadInConfig(); err != nil {
			return nil, errors.Wrapf(err, "failed to read config file '%s'", file)
		}
	}
	v.AutomaticEnv()

	cfg := new(Config)

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config file")
	}

	cfg.v = v
	return cfg, nil
}

// Save saves the Epinio config
func (c *Config) Save() error {
	c.v.Set("org", c.Org)

	err := os.MkdirAll(filepath.Dir(c.v.ConfigFileUsed()), 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create config dir '%s'", filepath.Dir(c.v.ConfigFileUsed()))
	}

	err = c.v.WriteConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to write config file '%s'", c.v.ConfigFileUsed())
	}

	return nil
}

func location() string {
	return viper.GetString("config-file")
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to stat file '%s'", path)
	}
}
