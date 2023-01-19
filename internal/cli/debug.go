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
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var ()

func init() {
	CmdDebug.AddCommand(CmdDebugTTY)
}

// CmdDebug implements the command: epinio debug
var CmdDebug = &cobra.Command{
	Hidden:        true,
	Use:           "debug",
	Short:         "Dev Tools",
	Long:          `Developer Tools. Hidden From Regular User.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

// CmdDebug implements the command: epinio debug tty
var CmdDebugTTY = &cobra.Command{
	Use:   "tty",
	Short: "Running In a Terminal?",
	Long:  `Running In a Terminal?`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Terminal")
		} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Cygwin/MSYS2 Terminal")
		} else {
			fmt.Println("Is Not Terminal")
		}
		return nil
	},
}
