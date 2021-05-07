package cli

import (
	"os"
	"strings"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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

		services, err := cmd.Flags().GetStringSlice("bind")
		if err != nil {
			return err
		}

		err = client.Push(args[0], path, services)
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
	flags.IntP("instances", "i", 1, "The number of desired instance for the application")
}
