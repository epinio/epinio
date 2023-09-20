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

package manifest

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

const (
	separator = ","
)

// UpdateRoutes updates the incoming manifest with information pulled from the --route option.
// Option information replaces any existing information.
func UpdateRoutes(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	routes, err := cmd.Flags().GetStringSlice("route")
	if err != nil {
		return manifest, errors.Wrap(err, "could not read option --route")
	}

	if len(routes) > 0 {
		manifest.Configuration.Routes = routes
	}

	clearRoutes, err := cmd.Flags().GetBool("clear-routes")
	if err != nil {
		return manifest, errors.Wrap(err, "could not read option --clear-routes")
	}
	if clearRoutes {
		// Note: This is not the nil slice.
		manifest.Configuration.Routes = []string{}
	}

	return manifest, nil
}

// UpdateBASN updates the incoming manifest with information pulled from the --builder,
// sources (--path, --git, --git-provider, and --container-image-url), --app-chart, and --name options.
// Option information replaces any existing information.
func UpdateBASN(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	var err error
	// BASN - Builder, AppChart, Source origin, Name

	// B:uilder - Retrieve from options
	manifest, err = UpdateBuilder(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// A:ppChart - Retrieve from options
	manifest, err = UpdateAppChart(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// S:ources - Retrieve from options
	manifest, err = UpdateSources(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// N:ame - Retrieve from options
	manifest, err = UpdateName(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

// UpdateBuilder updates the incoming manifest with information pulled from the --builder option
func UpdateBuilder(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	builderImage, err := cmd.Flags().GetString("builder-image")
	if err != nil {
		return manifest, errors.Wrap(err, "could not read option --builder-image")
	}

	// B:uilder - Replace

	if builderImage != "" {
		manifest.Staging.Builder = builderImage
	}

	return manifest, nil
}

// UpdateAppChart updates the incoming manifest with information pulled from the --app-chart option
func UpdateAppChart(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	appChart, err := cmd.Flags().GetString("app-chart")
	if err != nil {
		return manifest, errors.Wrap(err, "could not read option --app-chart")
	}

	// A:ppchart - Replace

	if appChart != "" {
		manifest.Configuration.AppChart = appChart
	}

	return manifest, nil
}

// UpdateSources updates the incoming manifest with information pulled from the sources
// (--path, --git, --git-provider, and --container-image-url) options
func UpdateSources(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	path, err := cmd.Flags().GetString("path")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --path")
	}

	git, err := cmd.Flags().GetString("git")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --git")
	}

	gitProvider, err := cmd.Flags().GetString("git-provider")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --git-provider")
	}

	container, err := cmd.Flags().GetString("container-image-url")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --container-image-url")
	}

	kind := models.OriginNone
	origins := 0

	if path != "" {
		kind = models.OriginPath
		origins++
	}

	if container != "" {
		kind = models.OriginContainer
		origins++
	}

	gitRef := &models.GitRef{}
	if git != "" {
		kind = models.OriginGit
		origins++

		if origins == 1 {
			pieces := strings.Split(git, separator)
			if len(pieces) > 2 {
				return manifest, errors.New("Bad --git reference git `" + git + "`, expected `repo?,rev?` as value")
			}
			if len(pieces) == 1 {
				gitRef.URL = git
			}
			if len(pieces) == 2 {
				gitRef.URL = pieces[0]
				gitRef.Revision = pieces[1]
			}

			// Standard provider (from git url), and conditional override by the user
			gitRef.Provider = gitProviderFromOriginURL(gitRef.URL)
			if gitProvider != "" {
				provider, err := models.GitProviderFromString(gitProvider)
				if err != nil {
					return manifest, errors.New("Bad --git-provider `" + gitProvider + "`")
				}
				gitRef.Provider = provider
			}
		}
	}

	if origins > 1 {
		return manifest, errors.New("Cannot use `--path`, `--git`, and `--container-image-url` options together")
	}

	// Resolve relative path to app sources, relative to CWD
	if kind == models.OriginPath && !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return manifest, errors.Wrap(err, "filesystem error")
		}
	}

	// S:ources - Replace

	if origins > 0 {
		manifest.Origin = models.ApplicationOrigin{}
		manifest.Origin.Kind = kind

		if path != "" {
			manifest.Origin.Path = path
		}
		if container != "" {
			manifest.Origin.Container = container
		}
		if git != "" {
			manifest.Origin.Git = gitRef
		}
	}

	return manifest, nil
}

// UpdateName updates the incoming manifest with information pulled from the --name option
func UpdateName(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --name")
	}

	// N:ame - Replace

	if name != "" {
		manifest.Name = name
	}

	return manifest, nil
}

// UpdateICE updates the incoming manifest with information pulled from the
// --bind, --env, and --instances options.
// Option information replaces any existing information.
func UpdateICE(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	var err error
	// ICE - Instances, Configurations, Environment

	// I:nstances - Retrieve from options
	manifest, err = UpdateInstances(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// C:onfigurations - Retrieve from options
	manifest, err = UpdateConfigurations(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// E:nvironment - Retrieve from options
	manifest, err = UpdateEnvironment(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	// ChartValues - Retrieve from options
	manifest, err = UpdateChartValues(manifest, cmd)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

// UpdateInstances updates the incoming manifest with information pulled from the --instances option
func UpdateInstances(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	instances, err := instances(cmd)
	if err != nil {
		return manifest, err
	}

	// I:nstances - Replace

	if instances != nil {
		manifest.Configuration.Instances = instances
	}
	// nil --> Default / No change
	// - AppCreate API will replace it with `v1.DefaultInstances`
	// - AppUpdate API will treat it as no op, i.e. keep current instances.

	return manifest, nil
}

// UpdateConfigurations updates the incoming manifest with information pulled from the --bind option
func UpdateConfigurations(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	boundConfigurations, err := cmd.Flags().GetStringSlice("bind")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --bind")
	}

	// C: Configurations - Replace

	if len(boundConfigurations) > 0 {
		boundConfigurations = helpers.UniqueStrings(boundConfigurations)
		manifest.Configuration.Configurations = boundConfigurations
	}

	return manifest, nil
}

// UpdateEnvironment updates the incoming manifest with information pulled from the --env option
func UpdateEnvironment(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	evAssignments, err := cmd.Flags().GetStringSlice("env")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --env")
	}

	environment := models.EnvVariableMap{}
	for _, assignment := range evAssignments {
		pieces := strings.SplitN(assignment, "=", 2)
		if len(pieces) < 2 {
			return manifest, errors.New("Bad --env assignment `" + assignment + "`, expected `name=value` as value")
		}
		environment[pieces[0]] = pieces[1]
	}

	// E:nvironment - Replace

	if len(environment) > 0 {
		manifest.Configuration.Environment = environment
	}

	return manifest, nil
}

// UpdateChartValues updates the incoming manifest with information pulled from the --chart-value option
func UpdateChartValues(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	evAssignments, err := cmd.Flags().GetStringSlice("chart-value")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --chart-value")
	}

	chartValues := models.ChartValueSettings{}
	for _, assignment := range evAssignments {
		pieces := strings.SplitN(assignment, "=", 2)
		if len(pieces) < 2 {
			return manifest, errors.New("Bad --chart-value `" + assignment + "`, expected `name=value` as value")
		}
		chartValues[pieces[0]] = pieces[1]
	}

	// E:nvironment - Replace

	if len(chartValues) > 0 {
		manifest.Configuration.Settings = chartValues
	}

	return manifest, nil
}

// Get reads the manifest at the spcified path into
// memory. Note that a missing file is not an error. It simply maps to
// an empty manifest.
func Get(manifestPath string) (models.ApplicationManifest, error) {

	// Empty manifest, for errors
	empty := models.ApplicationManifest{}

	manifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return empty, errors.Wrapf(err, "filesystem error")
	}

	manifestExists, err := fileExists(manifestPath)
	if err != nil {
		return empty, errors.Wrapf(err, "filesystem error")
	}

	defaultOrigin := models.ApplicationOrigin{
		Kind: models.OriginPath,
		Path: filepath.Dir(manifestPath),
	}

	// Base manifest, defaults
	// Note: Builder defaults to empty string - Insertion of Default builder happens server side.
	manifest := models.ApplicationManifest{
		Self:    "<<Defaults>>",
		Origin:  defaultOrigin,
		Staging: models.ApplicationStage{},
	}

	if !manifestExists {
		// Without manifest we simply provide the defaults for app sources and
		// builder.

		return manifest, nil
	}

	yamlFile, err := os.ReadFile(manifestPath)
	if err != nil {
		return empty, errors.Wrapf(err, "filesystem error")
	}

	// Modified manifest 2. Remove default origin - would clash with the unmarshalled
	// data. Will be added back later if no origin was specified by the manifest
	// itself.
	manifest.Origin = models.ApplicationOrigin{}

	err = yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		return empty, errors.Wrapf(err, "bad yaml")
	}

	// Verify that origin information is one-of only.

	manifest.Self = manifestPath

	origins := 0
	if manifest.Origin.Path != "" {
		manifest.Origin.Kind = models.OriginPath
		origins++
	}

	if manifest.Origin.Container != "" {
		manifest.Origin.Kind = models.OriginContainer
		origins++
	}

	if manifest.Origin.Git != nil && manifest.Origin.Git.URL != "" {
		manifest.Origin.Kind = models.OriginGit
		origins++
	}

	if origins > 1 {
		return empty, errors.New("Cannot use `path`, `git`, and `container` keys together")
	}

	// Add default location (manifest directory) back, if needed
	if origins == 0 {
		manifest.Origin = defaultOrigin
	}

	// Resolve relative path to app sources, relative to manifest file directory
	if manifest.Origin.Kind == models.OriginPath &&
		!filepath.IsAbs(manifest.Origin.Path) {
		manifest.Origin.Path = filepath.Join(
			filepath.Dir(manifestPath),
			manifest.Origin.Path)
	}

	return manifest, nil
}

// instances checks if the user provided an instance count. If they didn't, then we'll
// pass nil and either use the default or whatever is deployed in the cluster.
func instances(cmd *cobra.Command) (*int32, error) {
	var i *int32

	instances, err := cmd.Flags().GetInt32("instances")
	if err != nil {
		cmd.SilenceUsage = false
		return i, errors.Wrap(err, "could not read instances parameter")
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name == "instances" {
			n := int32(instances)
			i = &n
		}
	})

	return i, nil
}

// See also settings/settings.go
func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to stat file '%s'", path)
	}
}

func gitProviderFromOriginURL(theurl string) models.GitProvider {
	u, err := url.Parse(theurl)
	if err != nil {
		// A bad url will generate an issue on the server side which should tell us better
		// what is broken. Thus, swallow the error and return a semi-sensible provider.
		return models.ProviderGit
	}
	if u.Host == "github.com" {
		return models.ProviderGithub
	}
	if u.Host == "gitlab.com" {
		return models.ProviderGitlab
	}
	return models.ProviderGit
}
