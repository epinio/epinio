package config

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
)

var (
	defaultConfigFilePath = os.ExpandEnv("${HOME}/.config/epinio/config.yaml")
)

// Config represents a epinio config
type Config struct {
	Org      string `mapstructure:"namespace"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"pass"`
	API      string `mapstructure:"api"`
	WSS      string `mapstructure:"wss"`
	Certs    string `mapstructure:"certs"`
	Colors   bool   `mapstructure:"colors"`

	Location string // Origin of data, file which was loaded

	v   *viper.Viper
	log logr.Logger
}

// DefaultLocation returns the standard location for the configuration file
func DefaultLocation() string {
	return defaultConfigFilePath
}

// Load loads the Epinio config from the default location
func Load() (*Config, error) {
	return LoadFrom(location())
}

// LoadFrom loads the Epinio config from a specific file
func LoadFrom(file string) (*Config, error) {
	cfg := new(Config)

	log := tracelog.NewLogger().WithName(fmt.Sprintf("Config-%p", cfg)).V(3)
	log.Info("Loading", "from", file)

	v := viper.New()

	v.SetConfigType("yaml")
	v.SetConfigFile(file)
	v.SetEnvPrefix("EPINIO")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	v.SetDefault("namespace", "workspace")

	// Use empty defaults in viper to allow NeededOptions defaults to apply
	v.SetDefault("user", "")
	v.SetDefault("pass", "")
	v.SetDefault("api", "")
	v.SetDefault("wss", "")
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

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config file")
	}

	cfg.v = v
	cfg.Location = file

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
				InsecureSkipVerify: true, // nolint:gosec // Controlled by user option
			}

			http.DefaultTransport.(*http.Transport).TLSClientConfig = tlsInsecure
			websocket.DefaultDialer.TLSClientConfig = tlsInsecure
		} else {
			// nolint:gosec // Controlled by user option
			http.DefaultTransport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
			// websocket.DefaultDialer.TLSClientConfig refers to the same structure,
			// and the assignment has modified it also.
		}
	}

	if !cfg.Colors || viper.GetBool("no-colors") {
		color.NoColor = true
	}

	cfg.log = log
	log.Info("Loaded", "value", cfg.String())
	return cfg, nil
}

// Generates a string representation of the configuration (for debugging)
func (c *Config) String() string {
	return fmt.Sprintf(
		"namespace=(%s), user=(%s), pass=(%s), api=(%s), wss=(%s), color=(%v), @(%s)",
		c.Org, c.User, c.Password, c.API, c.WSS, c.Colors, c.Location)
}

// Save saves the Epinio config
func (c *Config) Save() error {
	c.v.Set("namespace", c.Org)
	c.v.Set("user", c.User)
	c.v.Set("pass", c.Password)
	c.v.Set("api", c.API)
	c.v.Set("wss", c.WSS)
	c.v.Set("certs", c.Certs)
	c.v.Set("colors", c.Colors)

	c.log.Info("Saving", "to", c.v.ConfigFileUsed())

	err := os.MkdirAll(filepath.Dir(c.v.ConfigFileUsed()), 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create config dir '%s'", filepath.Dir(c.v.ConfigFileUsed()))
	}

	err = c.v.WriteConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to write config file '%s'", c.v.ConfigFileUsed())
	}

	c.log.Info("Saved", "value", c.String())

	// Note: Install saves the config via ConfigUpdate. The newly
	// retrieved cert(s) have to be made available now, so that
	// creation of the default org can do proper verification.
	if c.Certs != "" {
		auth.ExtendLocalTrust(c.Certs)
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
