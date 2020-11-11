package app

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/suse/carrier/shim/app/configuration"
	"github.com/suse/carrier/shim/restapi"
)

var signalChannel = make(chan os.Signal, 1) // for trapping SIGHUP and friends

// App is the configuration state for the entire app.
type App struct {
	Config *configuration.Struct
	Log    zerolog.Logger

	Server *restapi.Server
}

// NewLogger returns a new zap logger
func NewLogger(logOutput io.Writer, cfg *configuration.Struct) (zerolog.Logger, error) {
	var logger zerolog.Logger
	if cfg.LogPretty {
		cw := zerolog.NewConsoleWriter()
		cw.Out = logOutput
		logger = zerolog.New(cw).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(logOutput).With().Timestamp().Logger()
	}
	zerolog.SetGlobalLevel(cfg.LogLevelParsed)

	return logger, nil
}
