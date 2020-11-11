//+build wireinject

package server

import (
	"io"

	"github.com/google/wire"
	"github.com/spf13/pflag"
	"github.com/suse/carrier/shim/app"
	"github.com/suse/carrier/shim/app/configuration"
)

func BuildApp(log io.Writer, flags *pflag.FlagSet) (*app.App, func(), error) {
	wire.Build(
		wire.Struct(new(app.App), "*"),
		app.NewLogger,
		configuration.NewConfig,
		ShimServer,
	)

	return &app.App{}, func() {}, nil
}
