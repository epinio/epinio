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

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/auth"
)

var (
	defaultSettingsFilePath = "epinio/settings.yaml"
)

const (
	envPassword       = "EPINIO_PASSWORD"
	envPasswordB64    = "EPINIO_PASSWORD_B64"
	envPasswordLegacy = "EPINIO_PASS"
	envAPIToken       = "EPINIO_API_TOKEN" //nolint:gosec // G101: env var name, not a credential
)

// Settings represents a epinio settings
type Settings struct {
	Namespace string       `mapstructure:"namespace"` // Currently targeted namespace
	User      string       `mapstructure:"user"`
	Password  string       `mapstructure:"pass"` // nolint:gosec // intentional auth field for API
	Token     TokenSetting `mapstructure:"token"`
	API       string       `mapstructure:"api"`
	WSS       string       `mapstructure:"wss"`
	Certs     string       `mapstructure:"certs"`
	Colors    bool         `mapstructure:"colors"`
	AppChart  string       `mapstructure:"appchart"` // Current default app chart (name)

	Location string // Origin of data, file which was loaded

	// passwordFromEnv is set when Password came from EPINIO_PASSWORD / EPINIO_PASSWORD_B64 / EPINIO_PASS.
	// filePassword is the decoded password from the settings file (if any), kept so Save does not
	// persist an environment override.
	passwordFromEnv bool
	filePassword    string

	// accessTokenFromEnv is set when the access token came from EPINIO_API_TOKEN.
	// fileToken is the token from the settings file (if any), kept so Save does not persist an
	// environment override.
	accessTokenFromEnv bool
	fileToken          TokenSetting

	v   *viper.Viper
	log logr.Logger
}

type TokenSetting struct {
	AccessToken  string    `json:"accesstoken" mapstructure:"accesstoken"`   // nolint:gosec // intentional auth field
	TokenType    string    `json:"tokentype,omitempty" mapstructure:"tokentype,omitempty"`
	RefreshToken string    `json:"refreshtoken,omitempty" mapstructure:"refreshtoken,omitempty"` // nolint:gosec // intentional auth field
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

	log := helpers.LoggerToLogr().WithName(fmt.Sprintf("Settings-%p", cfg)).V(3)
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

	// Snapshot file-backed secrets before binding environment overrides. BindEnv/AutomaticEnv
	// make later Gets return the env value, and Save must not persist those overrides.
	var filePassEncoded string
	if settingsExists {
		cfg.Location = file
		if err := v.ReadInConfig(); err != nil {
			return nil, errors.Wrapf(err, "failed to read settings file '%s'", file)
		}
		filePassEncoded = v.GetString("pass")
		if err := v.UnmarshalKey("token", &cfg.fileToken); err != nil {
			return nil, errors.Wrap(err, "failed to read token from settings file")
		}
	}

	// The token is a nested struct, so AutomaticEnv cannot resolve it. Bind it explicitly.
	if err := v.BindEnv("token.accesstoken", envAPIToken); err != nil {
		return nil, errors.Wrap(err, "failed to bind the API token")
	}
	v.AutomaticEnv()

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal settings file")
	}

	cfg.v = v
	cfg.filePassword = decodeFilePassword(filePassEncoded)

	if tokenFromEnv := os.Getenv(envAPIToken); tokenFromEnv != "" {
		cfg.accessTokenFromEnv = true
		// An explicitly supplied access token is not part of an OIDC refresh flow; drop the
		// leftover refresh material so a later Save cannot mix secrets from two sources.
		cfg.Token.RefreshToken = ""
		cfg.Token.Expiry = time.Time{}
	}

	if cfg.Certs != "" {
		auth.ExtendLocalTrust(cfg.Certs)
	}

	// Check skip-ssl-verification flag from viper (bound flag) or environment variable
	// The flag is bound in root.go, so we check the global viper instance
	skipSSLVerification := viper.GetBool("skip-ssl-verification")
	
	// Also check environment variables as fallback (viper might not have the flag value yet)
	// Check both EPINIO_SKIP_SSL_VERIFICATION and SKIP_SSL_VERIFICATION
	envSkipSSL := os.Getenv("EPINIO_SKIP_SSL_VERIFICATION")
	if envSkipSSL == "" {
		envSkipSSL = os.Getenv("SKIP_SSL_VERIFICATION")
	}
	if envSkipSSL == "true" || envSkipSSL == "1" {
		skipSSLVerification = true
	}
	
	log.V(1).Info("SSL verification check", 
		"skip-ssl-verification", skipSSLVerification,
		"viper-value", viper.GetBool("skip-ssl-verification"),
		"env-value", envSkipSSL)

	if skipSSLVerification {
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

	if err := cfg.resolvePassword(); err != nil {
		return cfg, err
	}

	cfg.log = log
	log.Info("Loaded", "value", cfg.String())
	return cfg, nil
}

// resolvePassword sets the final plaintext password. The dedicated environment variables are
// read here, after the unmarshal, so they stay out of viper's automatic binding and cannot
// interfere with the `pass` key of the settings file.
//
// EPINIO_PASSWORD is plaintext, EPINIO_PASSWORD_B64 is base64 encoded. The legacy EPINIO_PASS
// is the viper binding of the `pass` key, and thus base64 encoded as well. It stays as an
// undocumented alias of EPINIO_PASSWORD_B64.
func (c *Settings) resolvePassword() error {
	plain := os.Getenv(envPassword)
	encoded := os.Getenv(envPasswordB64)

	if legacy := os.Getenv(envPasswordLegacy); encoded == "" && legacy != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s is deprecated, use %s (base64) or %s (plaintext)\n",
			envPasswordLegacy, envPasswordB64, envPassword)
		encoded = legacy
	}

	if plain != "" && encoded != "" {
		return fmt.Errorf("%s and %s are both set, use only one", envPassword, envPasswordB64)
	}

	if plain != "" {
		c.passwordFromEnv = true
		c.Password = plain
		return nil
	}

	if encoded != "" {
		c.passwordFromEnv = true
	} else {
		// Neither variable is set, the password comes from the settings file.
		encoded = c.Password
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	c.Password = string(decodedPassword)

	return nil
}

// decodeFilePassword decodes a settings-file password. An empty or invalid value yields "".
// Invalid base64 from the file is ignored here; resolvePassword still reports decode errors
// for the live Password field.
func decodeFilePassword(encoded string) string {
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// ErrEnvOnly is returned when settings which come from the environment alone are to be saved.
// Callers wrap it with ErrEnvOnlyFor to name the variable the user should set instead.
var ErrEnvOnly = errors.New("settings come from the environment only, there is no settings file to save to")

// ErrEnvOnlyFor returns ErrEnvOnly naming the environment variable that should be set instead
// of writing a settings file.
func ErrEnvOnlyFor(envVar string) error {
	return fmt.Errorf("%w; set %s instead", ErrEnvOnly, envVar)
}

// EnvOnly reports whether the settings come from the environment alone: there is no settings
// file, but an API server is configured.
func (c *Settings) EnvOnly() bool {
	return c.Location == "" && c.API != ""
}

// AccessTokenFromEnv reports whether the access token came from EPINIO_API_TOKEN.
func (c *Settings) AccessTokenFromEnv() bool {
	return c.accessTokenFromEnv
}

// ClearEnvCredentialOverrides stops treating password/token as environment overrides so
// Save writes the current in-memory values. Call this after intentional credential changes
// (login, logout, OIDC login). Without it, Save would keep the previous file-backed secrets
// and drop the change when EPINIO_PASSWORD / EPINIO_API_TOKEN are set in the environment.
func (c *Settings) ClearEnvCredentialOverrides() {
	c.passwordFromEnv = false
	c.accessTokenFromEnv = false
}

// Origin describes where the settings come from, for display.
func (c *Settings) Origin() string {
	if c.EnvOnly() {
		return "<environment>"
	}
	return helpers.AbsPath(c.Location)
}

// SaveUnlessEnvOnly is Save for commands which only adjust a setting, like the current
// namespace. Writing a file for settings which come from the environment alone would persist
// the environment's credentials to disk. envVar is the EPINIO_* variable the user should set
// instead when the settings are environment-only.
func (c *Settings) SaveUnlessEnvOnly(envVar string) error {
	if c.EnvOnly() {
		return ErrEnvOnlyFor(envVar)
	}
	return c.Save()
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
	// Keep environment overrides out of settings.yaml for incidental saves (target,
	// colors, …). Intentional credential updates must call ClearEnvCredentialOverrides
	// first so login/logout/OIDC persist what the user set.
	if c.passwordFromEnv {
		c.v.Set("pass", base64.StdEncoding.EncodeToString([]byte(c.filePassword)))
	} else {
		c.v.Set("pass", base64.StdEncoding.EncodeToString([]byte(c.Password)))
	}
	if c.accessTokenFromEnv {
		c.v.Set("token", c.fileToken)
	} else {
		c.v.Set("token", c.Token)
	}
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
