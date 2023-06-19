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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdAppDelete implements the command: epinio app delete
var CmdAppDelete = &cobra.Command{
	Use:   "delete NAME1 [NAME2 ...]",
	Short: "Deletes one or more applications",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {

		filteredMatches := filteredMatchingFinder(args, toComplete, client.AppsMatching)
		return filteredMatches, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return errors.Wrap(err, "error reading option --all")
		}

		if all && len(args) > 0 {
			return errors.New("Conflict between --all and named applications")
		}
		if !all && len(args) == 0 {
			return errors.New("No applications specified for deletion")
		}

		err = client.Delete(cmd.Context(), args, all)
		if err != nil {
			return errors.Wrap(err, "error deleting app")
		}

		return nil
	},
}
