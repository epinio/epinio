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

// ValidArgsFunc is a shorthand type for cobra argument validation functions.
type ValidArgsFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

//counterfeiter:generate -header ../../../LICENSE_HEADER . NamespaceMatcher
type NamespaceMatcher interface {
	GetAPI() usercmd.APIClient
	NamespacesMatching(toComplete string) []string
}

//counterfeiter:generate -header ../../../LICENSE_HEADER . ConfigurationMatcher
type ConfigurationMatcher interface {
	GetAPI() usercmd.APIClient
	ConfigurationMatching(toComplete string) []string
}

//counterfeiter:generate -header ../../../LICENSE_HEADER . ConfigurationAppMatcher
type ConfigurationAppMatcher interface {
	GetAPI() usercmd.APIClient
	ConfigurationMatching(toComplete string) []string
	AppsMatching(toComplete string) []string
}

// NewNamespaceMatcherFunc returns a function returning list of matching namespaces from the
// provided partial command.  It only matches for the first command argument.
func NewNamespaceMatcherFunc(matcher NamespaceMatcher) ValidArgsFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matcher.GetAPI().DisableVersionWarning()

		matches := matcher.NamespacesMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// NewConfigurationMatcherFirstFunc returns a function returning list of matching configurations from the
// provided partial command.  It only matches for the first command argument.
func NewConfigurationMatcherFirstFunc(matcher ConfigurationMatcher) ValidArgsFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matcher.GetAPI().DisableVersionWarning()

		matches := matcher.ConfigurationMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// NewConfigurationMatcherAnyFunc returns a function returning a list of matching configurations
// from the provided partial command.  It matches for all command arguments
func NewConfigurationMatcherAnyFunc(matcher ConfigurationMatcher) ValidArgsFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		matcher.GetAPI().DisableVersionWarning()

		matches := FilteredMatchingFinder(args, toComplete, matcher.ConfigurationMatching)

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// NewConfigurationAppMatcherFunc returns a function returning a list of matching configurations and
// apps from the provided partial command.  It matches for the first (configurations) and second
// arguments (applications)
func NewConfigurationAppMatcherFunc(matcher ConfigurationAppMatcher) ValidArgsFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matcher.GetAPI().DisableVersionWarning()

		if len(args) == 1 {
			// #args == 1: app name.
			matches := matcher.AppsMatching(toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 0: configuration name.

		matches := matcher.ConfigurationMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// FilteredMatchingFinder uses the finder function to find the resources matching the given prefix.
// It then filters the matches, removing the provided args, and returns that as its result.
func FilteredMatchingFinder(args []string, prefix string, finder func(prefix string) []string) []string {
	// When services and apps have moved into this package the function can be made private again.

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
