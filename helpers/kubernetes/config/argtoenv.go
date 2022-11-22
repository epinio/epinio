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
