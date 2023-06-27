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
	"strings"

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

// matchingConfigurationFinder returns a list of configurations whose names match the provided
// partial command. It only matches for the first command argument
func matchingConfigurationFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.ConfigurationMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
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

// matchingNamespaceFinder returns a list of matching namespaces from the provided partial
// command. It only matches for the first command argument.
func matchingNamespaceFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.NamespacesMatching(toComplete)
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

// matchingServiceChartValueFinder returns a list of chart values from the chosen service class
// whose names match the provided partial name
func matchingServiceChartValueFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	matches := []string{}

	// We cannot complete beyond the name of the chart value.
	if strings.Contains(toComplete, "=") {
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// We cannot complete without a service class providing the available chart values
	if len(args) == 0 {
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	// We cannot complete if the specified service class is bogus. That is the same as having no
	// class at all, see above.
	catalogService, err := client.API.ServiceCatalogShow(args[0])
	if err != nil {
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// With the class retrieved we now can iterate over the settings the class makes available
	// and match to the given partial.
	for name := range catalogService.Settings {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name+"=")
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingServiceFinder returns a list of matching services from the provided partial command. It
// only matches for the first command argument.
func matchingServiceFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.ServiceMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// matchingCatalogFinder returns a list of matching catalogs from the provided partial command. It
// only matches for the first command argument.
func matchingCatalogFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	matches := client.CatalogMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// filteredMatchingFinder will use the finder func to find the resources with the prefix name
// It will then filter the matches removing the provided args
func filteredMatchingFinder(args []string, prefix string, finder func(prefix string) []string) []string {
	// map to check for already selected resources
	alreadyMatched := map[string]struct{}{}
	for _, resource := range args {
		alreadyMatched[resource] = struct{}{}
	}

	filteredMatches := []string{}

	matches := finder(prefix)
	for _, resource := range matches {
		// return only the not already matched resources
		if _, found := alreadyMatched[resource]; !found {
			filteredMatches = append(filteredMatches, resource)
		}
	}

	return filteredMatches
}
