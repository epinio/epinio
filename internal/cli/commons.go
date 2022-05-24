package cli

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ExitIfError is a short form of ExitfIfError, with a standard message
// It is currently not used
func ExitIfError(err error) {
	ExitfIfError(err, "an unexpected error occurred")
}

// ExitfIfError stops the application with an error exit code, after
// printing error and message, if there is an error.
func ExitfIfError(err error, message string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s: %w", message, err))
		os.Exit(1)
	}
}

// ExitfWithMessage stops the application with an error exit code,
// after formatting and printing the message.
// It is currently not used
func ExitfWithMessage(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

// CreateKubeClient returns a client for access to kubernetes
// It is currently not used
func CreateKubeClient(configPath string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	ExitfIfError(err, "an unexpected error occurred")

	clientset, err := kubernetes.NewForConfig(config)
	ExitfIfError(err, "an unexpected error occurred")

	return clientset
}

// matchingAppsFinder returns a list of matching apps from the provided partial command
func matchingAppsFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	app, err := usercmd.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := app.AppsMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingNamespaceFinder returns a list of matching namespaces from the provided partial command
func matchingNamespaceFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	app, err := usercmd.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := app.NamespacesMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingChartFinder returns a list of application charts whose names match the provided partial name
func matchingChartFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	app, err := usercmd.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// #args == 0: chart name.
	matches := app.ChartMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingServiceFinder returns a list of matching services from the provided partial command
func matchingServiceFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	app, err := usercmd.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := app.ServiceMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingCatalogFinder returns a list of matching catalogs from the provided partial command
func matchingCatalogFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	app, err := usercmd.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := app.CatalogMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}
