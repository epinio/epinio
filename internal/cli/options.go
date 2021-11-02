package cli

import (
	"strings"

	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

// instancesOption initializes the --instances/-i option for the provided command
func instancesOption(cmd *cobra.Command) {
	cmd.Flags().Int32P("instances", "i", application.DefaultInstances,
		"The number of instances the application should have")
}

func domainOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("domain", "d", []string{}, "Custom domain to use as the application's route (a subdomain of the default domain will be used if this is not set). Can be set multiple times to use multiple domains with the same application.")
}

// bindOption initializes the --bind/-b option for the provided command
func bindOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("bind", "b", []string{}, "services to bind immediately")
	// nolint:errcheck // Unable to handle error in init block this will be called from
	cmd.RegisterFlagCompletionFunc("bind",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// `cmd`, `args` are ignored.  `toComplete` is the option value entered so far.
			//
			// This is a StringSlice option. This means that the option value is a comma-
			// separated string of values.
			//
			// Completion has to happen only for the last segment in that string, i.e. after
			// the last comma.  Note that cobra does not feed us a slice, just the string.
			// We are responsible for splitting into segments, and expanding only the last
			// segment.

			ctx := cmd.Context()

			app, err := usercmd.New()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			values := strings.Split(toComplete, ",")
			if len(values) == 0 {
				// Nothing. Report all possible matches
				matches := app.ServiceMatching(ctx, toComplete)
				return matches, cobra.ShellCompDirectiveNoFileComp
			}

			// Expand the last segment. The returned matches are
			// the string with its last segment replaced by the
			// expansions for that segment.

			matches := []string{}
			for _, match := range app.ServiceMatching(ctx, values[len(values)-1]) {
				values[len(values)-1] = match
				matches = append(matches, strings.Join(values, ","))
			}

			return matches, cobra.ShellCompDirectiveDefault
		})
}

// envOption initializes the --env/-e option for the provided command
func envOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("env", "e", []string{}, "environment variables to be used")
}
