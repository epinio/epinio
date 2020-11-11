package configuration

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"github.com/volatiletech/abcweb/v5/abcconfig"
)

// Struct holds the configuration for the app.
type Struct struct {
	LogPretty      bool          `toml:"log_pretty" mapstructure:"log_pretty"`
	LogLevel       string        `toml:"log_level" mapstructure:"log_level"`
	LogLevelParsed zerolog.Level `toml:"-"`

	CF struct {
		Server struct {
			ListenInterface string `toml:"listen_interface" mapstructure:"listen_interface"`
			Port            int    `toml:"port" mapstructure:"port"`
		} `toml:"server" mapstructure:"server"`
	} `toml:"cf" mapstructure:"cf"`
}

// NewConfig creates the configuration by reading env & files
func NewConfig(flags *pflag.FlagSet) (*Struct, error) {
	var err error
	cfg := new(Struct)

	c := abcconfig.NewConfig("carrier-shim-cf")
	if _, err := c.Bind(flags, cfg); err != nil {
		return nil, errors.Wrap(err, "cannot bind app config")
	}

	if len(cfg.LogLevel) == 0 {
		cfg.LogLevel = zerolog.InfoLevel.String()
	}

	err = func() error {
		var err error
		cfg.LogLevelParsed, err = zerolog.ParseLevel(cfg.LogLevel)
		if err != nil {
			return errors.Wrapf(err, "log_level failed to parse")
		}

		if cfg.CF.Server.Port == 0 {
			return errors.New("cf.server.port must be non-zero")
		}

		return nil
	}()

	if err != nil {
		return nil, errors.Wrap(err, "failed to validate config file")
	}

	return cfg, nil
}
