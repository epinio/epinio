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

package cmd

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

type ValidArgsFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// NewNamespaceMatcherFunc returns a list of matching namespaces from the provided partial command.
// It only matches for the first command argument.
func NewNamespaceMatcherFunc(client *usercmd.EpinioClient) ValidArgsFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		client.API.DisableVersionWarning()

		matches := client.NamespacesMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}
