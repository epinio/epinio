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

// matchingAppsFinder returns a list of matching apps from the provided partial command. It only
// matches for the first command argument.
func matchingAppsFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.AppsMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingChartFinder returns a list of application charts whose names match the provided partial name
func matchingChartFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	// #args == 0: chart name.
	matches := client.ChartMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingGitconfigFinder returns a list of matching git configurations from the provided partial
// command. It only matches for the first command argument.
func matchingGitconfigFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.GitconfigsMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingRegistryFinder returns a list of export registries whose names match the provided partial name
func matchingRegistryFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	// #args == 0: registry name.
	matches := client.ExportregistryMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}
