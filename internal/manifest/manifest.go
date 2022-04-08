package manifest

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

	return manifest, nil
}

// UpdateBASN updates the incoming manifest with information pulled from the --builder,
// sources (--path, --git, and --container-imageurl), --app-chart, and --name options.
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
// (--path, --git, and --container-imageurl) options
func UpdateSources(manifest models.ApplicationManifest, cmd *cobra.Command) (models.ApplicationManifest, error) {
	path, err := cmd.Flags().GetString("path")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --name")
	}

	git, err := cmd.Flags().GetString("git")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --name")
	}

	container, err := cmd.Flags().GetString("container-image-url")
	if err != nil {
		return manifest, errors.Wrap(err, "failed to read option --name")
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
		boundConfigurations = uniqueStrings(boundConfigurations)
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

	yamlFile, err := ioutil.ReadFile(manifestPath)
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

// uniqueStrings process the string slice and returns a slice where
// duplicate strings are removed. The order of strings is not touched.
// It does not assume a specific order.
func uniqueStrings(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to stat file '%s'", path)
	}
}
