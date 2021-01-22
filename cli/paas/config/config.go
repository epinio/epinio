package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	defaultConfigFilePath = os.ExpandEnv("${HOME}/.config/carrier/config.yaml")
)

// Config represents a carrier config
type Config struct {
	GiteaNamespace           string `mapstructure:"gitea_namespace"`
	GiteaProtocol            string `mapstructure:"gitea_protocol"`
	EiriniWorkloadsNamespace string `mapstructure:"eirini_workloads_namespace"`
	Org                      string `mapstructure:"org"`

	v *viper.Viper
}

// Load loads the Carrier config
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	file := defaultConfigFilePath
	if f, err := flags.GetString("config-file"); err == nil && f != "" {
		file = f
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to get config flag")
	}

	v.SetConfigType("yaml")
	v.SetConfigFile(file)
	v.SetEnvPrefix("CARRIER")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	v.SetDefault("gitea_namespace", "gitea")
	v.SetDefault("gitea_protocol", "http")
	v.SetDefault("eirini_workloads_namespace", "eirini-workloads")
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

// Save saves the Carrier config
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

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to stat file '%s'", path)
	}
}
