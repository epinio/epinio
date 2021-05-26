package cli

import (
	"os"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var ()

func init() {
	CmdPush.Flags().StringSliceP("bind", "b", []string{}, "services to bind immediately")
	CmdPush.RegisterFlagCompletionFunc("bind", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// `cmd`, `args` are ignored.
		// `toComplete` is the option value entered so far.
		// This is a StringSlice option.
		// This means that the option value is a comma-separated string of values.
		// Completion has to happen only for the last segment in that string, i.e. after the last comma.
		// Note that cobra does not feed us a slice, just the string.
		// We are responsible for splitting into segments, and expanding only the last segment.

		app, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		values := strings.Split(toComplete, ",")
		if len(values) == 0 {
			// Nothing. Report all possible matches
			matches := app.ServiceMatching(toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// Expand the last segment. The returned matches are
		// the string with its last segment replaced by the
		// expansions for that segment.

		matches := []string{}
		for _, match := range app.ServiceMatching(values[len(values)-1]) {
			values[len(values)-1] = match
			matches = append(matches, strings.Join(values, ","))
		}

		return matches, cobra.ShellCompDirectiveDefault
	})
}

// instances checks if the user provided an instance count. If they didn't, then we'll
// pass nil and either use the default or whatever is deployed in the cluster.
func instances(cmd *cobra.Command) (*int32, error) {
	var i *int32
	instances, err := cmd.Flags().GetInt32("instances")
	if err != nil {
		return i, errors.Wrap(err, "could not read instances parameter")
	}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name == "instances" {
			n := int32(instances)
			i = &n
		}
	})
	return i, nil
}

// CmdPush implements the epinio push command
var CmdPush = &cobra.Command{
	Use:   "push NAME [PATH_TO_APPLICATION_SOURCES]",
	Short: "Push an application from the specified directory, or the current working directory",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		var path string
		if len(args) == 1 {
			path, err = os.Getwd()
			if err != nil {
				return errors.Wrap(err, "error pushing app")
			}
		} else {
			path = args[1]
		}

		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "path not accessible")
		}

		i, err := instances(cmd)
		if err != nil {
			return err
		}
		params := clients.PushParams{
			Instances: i,
		}

		services, err := cmd.Flags().GetStringSlice("bind")
		if err != nil {
			return err
		}
		params.Services = services

		err = client.Push(args[0], path, params)
		if err != nil {
			return errors.Wrap(err, "error pushing app to server")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	flags := CmdPush.Flags()
	flags.Int32P("instances", "i", v1.DefaultInstances, "The number of desired instances for the application, default only applies to new deployments")
}
