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

// NewClientSyncCmd implements the command: epinio client-sync
func NewClientSyncCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:   "client-sync",
		Short: "Downloads a client binary matching the currently logged server",
		Long:  `Synchronizes the epinio client with the server by downloading the matching binary and replacing the current one.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ClientSync()
			if err != nil {
				return errors.Wrap(err, "error syncing the Epinio client")
			}
			return nil
		},
	}
}
