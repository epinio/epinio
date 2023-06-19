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

package cli

import (
	"strings"

	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/spf13/cobra"
)

// instancesOption initializes the --instances/-i option for the provided command
func instancesOption(cmd *cobra.Command) {
	cmd.Flags().Int32P("instances", "i", application.DefaultInstances,
		"The number of instances the application should have")
}

func routeOption(cmd *cobra.Command) {
	cmd.Flags().BoolP("clear-routes", "z", false, "clear routes / no routes")
	cmd.Flags().StringSliceP("route", "r", []string{}, "Custom route to use for the application (a subdomain of the default domain will be used if this is not set). Can be set multiple times to use multiple routes with the same application.")
}

// bindOption initializes the --bind/-b option for the provided command
func bindOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("bind", "b", []string{}, "configurations to bind immediately")
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

			values := strings.Split(toComplete, ",")
			if len(values) == 0 {
				// Nothing. Report all possible matches
				matches := client.ConfigurationMatching(toComplete)
				return matches, cobra.ShellCompDirectiveNoFileComp
			}

			// Expand the last segment. The returned matches are
			// the string with its last segment replaced by the
			// expansions for that segment.

			matches := []string{}
			for _, match := range client.ConfigurationMatching(values[len(values)-1]) {
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

// chartValueOption initializes the --chartValue/-c option for the provided command
func chartValueOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("chart-value", "v", []string{}, "chart customization to be used")
}
