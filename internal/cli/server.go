package cli

import (
	"net"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suse/carrier/internal/api"
)

func init() {
	flags := CmdServer.Flags()
	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	viper.BindPFlag("port", flags.Lookup("port"))
	viper.BindEnv("port", "PORT")
}

// CmdServer implements the carrier server command
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "starts the Carrier server. You can connect to it using either your browser or the Carrier client.",
	RunE: func(cmd *cobra.Command, args []string) error {
		listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(viper.GetInt("port")))
		if err != nil {
			return err
		}

		return api.StartServer(listener)
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
