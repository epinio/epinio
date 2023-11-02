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
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func bindFlag(cmd *cobra.Command, key string) {
	err := viper.BindPFlag(key, cmd.Flags().Lookup(key))
	if err != nil {
		log.Fatal(err)
	}
}

func bindFlagCompletionFunc(cmd *cobra.Command, key string, fn FlagCompletionFunc) {
	err := cmd.RegisterFlagCompletionFunc(key, fn)
	if err != nil {
		log.Fatal(err)
	}
}

type FlagCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func NewStaticFlagsCompletionFunc[T ~string](allowedValues []T) FlagCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		matches := []string{}

		for _, allowed := range allowedValues {
			if strings.HasPrefix(string(allowed), toComplete) {
				matches = append(matches, string(allowed))
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}
