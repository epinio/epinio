package cli

import (
	"context"
	"sync"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/termui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ()

// CmdApp implements the epinio -app command
var CmdApp = &cobra.Command{
	Use:           "app",
	Aliases:       []string{"apps"},
	Short:         "Epinio application features",
	Long:          `Manage epinio application`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdApp.AddCommand(CmdAppShow)
	CmdApp.AddCommand(CmdAppList)
	CmdApp.AddCommand(CmdDeleteApp)
	CmdApp.AddCommand(CmdPush)
}

// CmdAppList implements the epinio `apps list` command
var CmdAppList = &cobra.Command{
	Use:   "list",
	Short: "Lists all applications",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Target remote epinio server instead of starting one
		port := viper.GetInt("port")
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		ui := termui.NewUI()
		srv, listeningPort, err := startEpinioServer(httpServerWg, port, ui)
		if err != nil {
			return err
		}

		// TODO: NOTE: This is a hack until the server is running inside the cluster
		cmd.Flags().String("server-url", "http://127.0.0.1:"+listeningPort, "")

		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Apps()
		if err != nil {
			return errors.Wrap(err, "error listing apps")
		}

		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		httpServerWg.Wait()

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdAppShow implements the epinio `apps show` command
var CmdAppShow = &cobra.Command{
	Use:   "show NAME",
	Short: "Describe the named application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Target remote epinio server instead of starting one
		port := viper.GetInt("port")
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		ui := termui.NewUI()
		srv, listeningPort, err := startEpinioServer(httpServerWg, port, ui)
		if err != nil {
			return err
		}

		// TODO: NOTE: This is a hack until the server is running inside the cluster
		cmd.Flags().String("server-url", "http://127.0.0.1:"+listeningPort, "")

		client, err := clients.NewEpinioClient(cmd.Flags())

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppShow(args[0])
		if err != nil {
			return errors.Wrap(err, "error listing apps")
		}

		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		httpServerWg.Wait()

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
