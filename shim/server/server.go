package server

import (
	"github.com/go-openapi/loads"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/suse/carrier/shim/app/configuration"
	"github.com/suse/carrier/shim/restapi"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf"
)

// ShimServer returns a shim server
func ShimServer(logger zerolog.Logger, cfg *configuration.Struct) (*restapi.Server, func(), error) {
	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		logger.Fatal().Err(err)
	}

	api := carrier_shim_cf.NewCloudFoundryAPI(swaggerSpec)
	server := restapi.NewServer(api)
	server.Port = cfg.CF.Server.Port
	server.Host = cfg.CF.Server.ListenInterface

	logger.Info().Msgf("Will be listening on '%s:%d'.", server.Host, server.Port)

	parser := flags.NewParser(server, flags.Default)
	parser.ShortDescription = "CF API"
	parser.LongDescription = "This is the specification for a Cloud Foundry server.\n"

	server.ConfigureAPI()

	return server, func() {}, nil
}
