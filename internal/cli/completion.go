package cli

import (
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
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletion(os.Stdout)
		}
	},
}
