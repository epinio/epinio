package cli

import (
	"os"
	"path/filepath"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

func init() {
	CmdAppPush.Flags().String("builder-image", "paketobuildpacks/builder:full", "Paketo builder image to use for staging")

	// The following options override manifest data
	CmdAppPush.Flags().StringP("git", "g", "", "Git repository and revision of sources")
	CmdAppPush.Flags().String("container-image-url", "", "Container image url for the app workload image")
	CmdAppPush.Flags().StringP("name", "n", "", "Application name.")
	CmdAppPush.Flags().StringP("path", "p", "", "Path to application sources.")

	bindOption(CmdAppPush)
	envOption(CmdAppPush)
	instancesOption(CmdAppPush)
}

// CmdAppPush implements the command: epinio app push
var CmdAppPush = &cobra.Command{
	Use:   "push [flags] [PATH_TO_APPLICATION_MANIFEST]",
	Short: "Push an application declared in the specified manifest",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
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

		m, err = manifest.UpdateISE(m, cmd)
		if err != nil {
			return err
		}

		m, err = manifest.UpdateSN(m, cmd)
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

		builderImage, err := cmd.Flags().GetString("builder-image")
		if err != nil {
			return errors.Wrap(err, "could not read option --builder-image")
		}

		if m.Origin.Kind == models.OriginPath {
			if _, err := os.Stat(m.Origin.Path); err != nil {
				// Path issue is user error. Show usage
				cmd.SilenceUsage = false
				return errors.Wrap(err, "path not accessible")
			}
		}

		params := usercmd.PushParams{
			BuilderImage:        builderImage,
			ApplicationManifest: m,
		}

		err = client.Push(cmd.Context(), params)
		if err != nil {
			return errors.Wrap(err, "error pushing app to server")
		}

		return nil
	},
}
