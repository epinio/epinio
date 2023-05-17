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
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

func init() {
	// The following options override manifest data
	CmdAppPush.Flags().StringP("git", "g", "", "Git repository and revision of sources separated by comma (e.g. GIT_URL,REVISION)")
	CmdAppPush.Flags().String("container-image-url", "", "Container image url for the app workload image")
	CmdAppPush.Flags().StringP("name", "n", "", "Application name. (mandatory if no manifest is provided)")
	CmdAppPush.Flags().StringP("path", "p", "", "Path to application sources.")
	CmdAppPush.Flags().String("builder-image", "", "Paketo builder image to use for staging")

	CmdAppPush.Flags().String("app-chart", "", "App chart to use for deployment")
	err := CmdAppPush.RegisterFlagCompletionFunc("app-chart", matchingChartFinder)
	checkErr(err)

	CmdAppPush.Flags().String("git-provider", "", "Git provider code (default 'git')")
	err = CmdAppPush.RegisterFlagCompletionFunc("git-provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			matches := []string{}
			for _, candidate := range models.ValidProviders {
				if strings.HasPrefix(string(candidate), toComplete) {
					matches = append(matches, string(candidate))
				}
			}
			return matches, cobra.ShellCompDirectiveDefault
		})
	checkErr(err)

	routeOption(CmdAppPush)
	bindOption(CmdAppPush)
	envOption(CmdAppPush)
	chartValueOption(CmdAppPush)
	instancesOption(CmdAppPush)
}

// CmdAppPush implements the command: epinio app push
var CmdAppPush = &cobra.Command{
	Use:   "push [flags] [PATH_TO_APPLICATION_MANIFEST]",
	Short: "Push an application declared in the specified manifest",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		// Syntax:
		//   - push [flags] [PATH-TO-MANIFEST-FILE]

		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "working directory not accessible")
		}

		var manifestPath string

		if len(args) == 1 {
			manifestPath = args[0]
		} else {
			manifestPath = filepath.Join(wd, "epinio.yml")
		}

		m, err := manifest.Get(manifestPath)
		if err != nil {
			cmd.SilenceUsage = false
			return errors.Wrap(err, "Manifest error")
		}

		m, err = manifest.UpdateICE(m, cmd)
		if err != nil {
			return err
		}

		m, err = manifest.UpdateBASN(m, cmd)
		if err != nil {
			return err
		}

		m, err = manifest.UpdateRoutes(m, cmd)
		if err != nil {
			return err
		}

		// Final manifest verify: Name is specified

		if m.Name == "" {
			cmd.SilenceUsage = false
			return errors.New("Name required, not found in manifest nor options")
		}

		// Final completion: Without origin fall back to working directory

		if m.Origin.Kind == models.OriginNone {
			m.Origin.Kind = models.OriginPath
			m.Origin.Path = wd
		}

		if m.Origin.Kind == models.OriginPath {
			if _, err := os.Stat(m.Origin.Path); err != nil {
				// Path issue is user error. Show usage
				cmd.SilenceUsage = false
				return errors.Wrap(err, "path not accessible")
			}
		}

		params := usercmd.PushParams{
			ApplicationManifest: m,
		}

		err = client.Push(cmd.Context(), params)
		if err != nil {
			return errors.Wrap(err, "error pushing app to server")
		}

		return nil
	},
}
