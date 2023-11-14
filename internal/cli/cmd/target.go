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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewTargetCmd returns a new 'epinio target' command
func NewTargetCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:               "target [namespace]",
		Short:             "Targets an epinio-controlled namespace.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: FirstArgValidator(client.NamespacesMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			err := client.Target(namespace)
			if err != nil {
				return errors.Wrap(err, "failed to set target")
			}

			return nil
		},
	}
}
