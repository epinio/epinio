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

package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddEnvToUsage adds env variables to help
func AddEnvToUsage(cmd *cobra.Command, argToEnv map[string]string) {
	for arg, env := range argToEnv {
		err := viper.BindEnv(arg, env)
		checkErr(err)

		flag := cmd.Flag(arg)

		if flag != nil {
			// add environment variable to the description
			flag.Usage = fmt.Sprintf("(%s) %s", env, flag.Usage)
		}
	}
}
