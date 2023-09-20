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

package settings

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
)

var (
	defaultSettingsFilePath = "epinio/settings.yaml"
)

// Settings represents a epinio settings
type Settings struct {
	Namespace string       `mapstructure:"namespace"` // Currently targeted namespace
	User      string       `mapstructure:"user"`
	Password  string       `mapstructure:"pass"`
	Token     TokenSetting `mapstructure:"token"`
	API       string       `mapstructure:"api"`
	WSS       string       `mapstructure:"wss"`
	Certs     string       `mapstructure:"certs"`
	Colors    bool         `mapstructure:"colors"`
	AppChart  string       `mapstructure:"appchart"` // Current default app chart (name)

	Location string // Origin of data, file which was loaded

	v   *viper.Viper
	log logr.Logger
}

type TokenSetting struct {
	AccessToken  string    `json:"accesstoken" mapstructure:"accesstoken"`
	TokenType    string    `json:"tokentype,omitempty" mapstructure:"tokentype,omitempty"`
	RefreshToken string    `json:"refreshtoken,omitempty" mapstructure:"refreshtoken,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty" mapstructure:"expiry,omitempty"`
}

// DefaultLocation returns the standard location for the settings file
func DefaultLocation() (string, error) {
	return xdg.ConfigFile(defaultSettingsFilePath)
}

// Load loads the Epinio settings from the default location
func Load() (*Settings, error) {
	return LoadFrom(location())
}

// LoadFrom loads the Epinio settings from a specific file
func LoadFrom(file string) (*Settings, error) {
	cfg := new(Settings)

	log := tracelog.NewLogger().WithName(fmt.Sprintf("Settings-%p", cfg)).V(3)
	log.Info("Loading", "from", file)

	v := viper.New()

	v.SetConfigType("yaml")
	v.SetConfigFile(file)
	v.SetEnvPrefix("EPINIO")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	v.SetDefault("namespace", "workspace")
	v.SetDefault("appchart", "")

	// Use empty defaults in viper to allow NeededOptions defaults to apply
	v.SetDefault("user", "")
	v.SetDefault("pass", "")
	v.SetDefault("api", "")
	v.SetDefault("wss", "")
	v.SetDefault("certs", "")
	v.SetDefault("colors", true)

	settingsExists, err := fileExists(file)
	if err != nil {
		return nil, errors.Wrapf(err, "filesystem error")
	}

	if settingsExists {
		cfg.Location = file
		if err := v.ReadInConfig(); err != nil {
			return nil, errors.Wrapf(err, "failed to read settings file '%s'", file)
		}
	}
	v.AutomaticEnv()

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal settings file")
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
				InsecureSkipVerify: true, // nolint:gosec // Controlled by user option
			}

			http.DefaultTransport.(*http.Transport).TLSClientConfig = tlsInsecure
		} else {
			// nolint:gosec // Controlled by user option
			http.DefaultTransport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
		}
		websocket.DefaultDialer.TLSClientConfig = http.DefaultTransport.(*http.Transport).TLSClientConfig.Clone()
	}

	if !cfg.Colors || viper.GetBool("no-colors") {
		color.NoColor = true
	}

	// Decode base64 password
	decodedPassword, err := base64.StdEncoding.DecodeString(cfg.Password)
	if err != nil {
		return cfg, err
	}
	cfg.Password = string(decodedPassword)

	cfg.log = log
	log.Info("Loaded", "value", cfg.String())
	return cfg, nil
}

// String generates a string representation of the settings (for debugging)
func (c *Settings) String() string {
	return fmt.Sprintf(
		"namespace=(%s), user=(%s), pass=(%s), access_token=(%v), api=(%s), wss=(%s), color=(%v), appchart=(%v), @(%s)",
		c.Namespace, c.User, c.Password, c.Token, c.API, c.WSS, c.Colors, c.AppChart, c.Location)
}

// Save saves the Epinio settings
func (c *Settings) Save() error {
	c.v.Set("namespace", c.Namespace)
	c.v.Set("appchart", c.AppChart)
	c.v.Set("user", c.User)
	c.v.Set("pass", base64.StdEncoding.EncodeToString([]byte(c.Password)))
	c.v.Set("token", c.Token)
	c.v.Set("api", c.API)
	c.v.Set("wss", c.WSS)
	c.v.Set("certs", c.Certs)
	c.v.Set("colors", c.Colors)

	c.log.Info("Saving", "to", c.v.ConfigFileUsed())

	err := os.MkdirAll(filepath.Dir(c.v.ConfigFileUsed()), 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create settings dir '%s'", filepath.Dir(c.v.ConfigFileUsed()))
	}

	err = c.v.WriteConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to write settings file '%s'", c.v.ConfigFileUsed())
	}

	c.log.Info("Saved", "value", c.String())

	// Note: Install saves the settings via SettingsUpdate. The newly
	// retrieved cert(s) have to be made available now, so that
	// creation of the default org can do proper verification.
	if c.Certs != "" {
		auth.ExtendLocalTrust(c.Certs)
	}

	return nil
}

func location() string {
	return viper.GetString("settings-file")
}

// See also manifest/manifest.go
func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to stat file '%s'", path)
	}
}
