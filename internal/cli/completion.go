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
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// CmdCompletion represents the completion command
var CmdCompletion = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script for a shell",
	Long: `To load completions:

Bash:

$ source <(epinio completion bash)

# To load completions for each session, execute once:
Linux:
  $ epinio completion bash > /etc/bash_completion.d/epinio
MacOS:
  $ epinio completion bash > /usr/local/etc/bash_completion.d/epinio

ATTENTION:
    The generated script requires the bash-completion package.
    See https://kubernetes.io/docs/tasks/tools/install-kubectl/#enabling-shell-autocompletion
    for information on how to install and activate it.

Zsh:

# If shell completion is not already enabled in your environment you will need
# to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

# To load completions for each session, execute once:
$ epinio completion zsh > "${fpath[1]}/_epinio"

# You will need to start a new shell for this setup to take effect.

Fish:

$ epinio completion fish | source

# To load completions for each session, execute once:
$ epinio completion fish > ~/.config/fish/completions/epinio.fish

Powershell:

PS> epinio completion powershell | Out-String | Invoke-Expression

# To load completions for every new session, run:
PS> epinio completion powershell > epinio.ps1
# and source this file from your powershell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletion(os.Stdout)
		}
		return errors.New("Internal error: Invalid shell")
	},
}
