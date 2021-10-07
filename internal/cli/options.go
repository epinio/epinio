package cli

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// instancesOption initializes the --instances/-i option for the provided command
func instancesOption(cmd *cobra.Command) {
	cmd.Flags().Int32P("instances", "i", application.DefaultInstances,
		"The number of instances the application should have")
}

// bindOption initializes the --bind/-b option for the provided command
func bindOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("bind", "b", []string{}, "services to bind immediately")
	// nolint:errcheck // Unable to handle error in init block this will be called from
	cmd.RegisterFlagCompletionFunc("bind",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// `cmd`, `args` are ignored.  `toComplete` is the option value entered so far.
			//
			// This is a StringSlice option. This means that the option value is a comma-
			// separated string of values.
			//
			// Completion has to happen only for the last segment in that string, i.e. after
			// the last comma.  Note that cobra does not feed us a slice, just the string.
			// We are responsible for splitting into segments, and expanding only the last
			// segment.

			ctx := cmd.Context()

			app, err := usercmd.New()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			values := strings.Split(toComplete, ",")
			if len(values) == 0 {
				// Nothing. Report all possible matches
				matches := app.ServiceMatching(ctx, toComplete)
				return matches, cobra.ShellCompDirectiveNoFileComp
			}

			// Expand the last segment. The returned matches are
			// the string with its last segment replaced by the
			// expansions for that segment.

			matches := []string{}
			for _, match := range app.ServiceMatching(ctx, values[len(values)-1]) {
				values[len(values)-1] = match
				matches = append(matches, strings.Join(values, ","))
			}

			return matches, cobra.ShellCompDirectiveDefault
		})
}

// envOption initializes the --env/-e option for the provided command
func envOption(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("env", "e", []string{}, "environment variables to be used")
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

// appConfiguration processes the `--bind`, `--env`, and `--instances`
// options and returns config for installing and updating applications.
func appConfiguration(cmd *cobra.Command, manifestDir string) (models.ApplicationUpdateRequest, error) {
	result := models.ApplicationUpdateRequest{}

	// Read requested configuration from a manifest file, if present.

	if manifestDir != "" {
		manifestPath := filepath.Join(manifestDir, "epinio.yml")
		manifestExists, err := fileExists(manifestPath)
		if err != nil {
			return result, errors.Wrapf(err, "filesystem error")
		}
		if manifestExists {
			var manifest models.ApplicationManifest

			yamlFile, err := ioutil.ReadFile(manifestPath)
			if err != nil {
				return result, errors.Wrapf(err, "filesystem error")
			}
			err = yaml.Unmarshal(yamlFile, &manifest)
			if err != nil {
				return result, errors.Wrapf(err, "bad yaml")
			}

			if manifest.Instances != nil {
				result.Instances = manifest.Instances
			}
			if len(manifest.Services) > 0 {
				sort.Strings(manifest.Services)
				result.Services = manifest.Services
			}
			if manifest.Environment != nil {
				for name, value := range manifest.Environment {
					result.Environment = append(result.Environment, models.EnvVariable{
						Name:  name,
						Value: value,
					})
				}
			}
		}
	}

	// Use option information to override/extend (parts) of the manifest.

	instances, err := instances(cmd)
	if err != nil {
		return result, err
	}

	services, err := cmd.Flags().GetStringSlice("bind")
	if err != nil {
		return result, errors.Wrap(err, "failed to read option --bind")
	}

	assignments, err := cmd.Flags().GetStringSlice("env")
	if err != nil {
		return result, errors.Wrap(err, "failed to read option --env")
	}

	ev := models.EnvVariableList{}
	for _, assignment := range assignments {
		pieces := strings.Split(assignment, "=")
		if len(pieces) != 2 {
			return result, errors.New("Bad --env assignment `" + assignment + "`, expected `name=value` as value")
		}
		ev = append(ev, models.EnvVariable{
			Name:  pieces[0],
			Value: pieces[1],
		})
	}

	// From here on out errors cannot happen anymore. Just filling
	// the structure with the extracted information.
	if instances != nil {
		result.Instances = instances
	}
	// nil --> Default / No change
	// - AppCreate API will replace it with `v1.DefaultInstances`
	// - AppUpdate API will treat it as no op, i.e. keep current instances.

	services = uniqueStrings(services)
	if len(services) > 0 {
		sort.Strings(services)
		result.Services = services
	}

	if len(ev) > 0 {
		result.Environment = ev
	}

	return result, nil
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
