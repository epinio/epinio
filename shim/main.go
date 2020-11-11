package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/shim/server"
	"github.com/volatiletech/abcweb/abcconfig"
)

//go:generate wire

var (
	version   = "unknown"
	buildTime = "unknown"
)

func main() {
	// Display the version hash and build time
	args := os.Args
	if len(args) == 2 && args[1] == "--version" {
		fmt.Println(fmt.Sprintf("Version: %q, built on %s.", version, buildTime))
		return
	}

	// Setup the cli
	root := rootSetup()

	if err := root.Execute(); err != nil {
		fmt.Println("root command execution failed:", err)
		os.Exit(1)
	}
}

func runRootCmd(cmd *cobra.Command, args []string) {
	a, cleanup, err := server.BuildApp(os.Stderr, cmd.Flags())
	if err != nil {
		fmt.Println("failed to initialize application:", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := a.Server.Serve(); err != nil {
		a.Log.Err(err)
	}
}

// rootSetup sets up the root cobra command
func rootSetup() *cobra.Command {
	root := &cobra.Command{
		Use: "shield [flags]",
		Run: runRootCmd,
	}

	// Register the cmd-line flags for --help output
	root.Flags().AddFlagSet(abcconfig.NewFlagSet())

	return root
}
