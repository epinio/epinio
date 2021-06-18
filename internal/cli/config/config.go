package config

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/epinio/epinio/internal/auth"
)

var (
	defaultConfigFilePath = os.ExpandEnv("${HOME}/.config/epinio/config.yaml")
)

// Config represents a epinio config
type Config struct {
	Org      string `mapstructure:"org"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"pass"`
	Certs    string `mapstructure:"certs"`
	Colors   bool   `mapstructure:"colors"`

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

	v.SetDefault("org", "workspace")

	// Use empty defaults in viper to allow NeededOptions defaults to apply
	v.SetDefault("user", "")
	v.SetDefault("pass", "")
	v.SetDefault("certs", "")
	v.SetDefault("colors", true)

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

	if cfg.Certs != "" {
		auth.ExtendLocalTrust(cfg.Certs)
	}

	if viper.GetBool("skip-ssl-verification") {
		// Note: This has to work regardless of if `ExtendLocalTrust` was invoked or not.
		// I.e. the `TLSClientConfig` of default http transport and default dialer may or
		// may not be nil. Actually we can assume that either both are nil, or none, and
		// further, if none are nil, they point to the same structure.

		if http.DefaultTransport.(*http.Transport).TLSClientConfig == nil {
			tlsInsecure := &tls.Config{
				InsecureSkipVerify: true,
			}

			http.DefaultTransport.(*http.Transport).TLSClientConfig = tlsInsecure
			websocket.DefaultDialer.TLSClientConfig = tlsInsecure
		} else {
			http.DefaultTransport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
			// websocket.DefaultDialer.TLSClientConfig refers to the same structure,
			// and the assignment has modified it also.
		}
	}

	if !cfg.Colors || viper.GetBool("no-colors") {
		color.NoColor = true
	}

	return cfg, nil
}

// Save saves the Epinio config
func (c *Config) Save() error {
	c.v.Set("org", c.Org)
	c.v.Set("user", c.User)
	c.v.Set("pass", c.Password)
	c.v.Set("certs", c.Certs)
	c.v.Set("colors", c.Colors)

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
