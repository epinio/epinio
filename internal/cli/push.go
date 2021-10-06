package cli

import (
	"os"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

func init() {
	CmdAppPush.Flags().String("builder-image", "paketobuildpacks/builder:full", "paketo builder image to use for staging")
	CmdAppPush.Flags().String("git", "", "git revision of sources. PATH becomes repository location")
	CmdAppPush.Flags().String("docker-image-url", "", "docker image url for the app workload image")

	bindOption(CmdAppPush)
	envOption(CmdAppPush)
	instancesOption(CmdAppPush)
}

// CmdAppPush implements the command: epinio app push
var CmdAppPush = &cobra.Command{
	Use:   "push NAME [URL|PATH_TO_APPLICATION_SOURCES]",
	Short: "Push an application from the specified directory, or the current working directory",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		gitRevision, err := cmd.Flags().GetString("git")
		if err != nil {
			return errors.Wrap(err, "could not read option --git")
		}

		dockerImageURL, err := cmd.Flags().GetString("docker-image-url")
		if err != nil {
			return errors.Wrap(err, "could not read option --docker-image-url")
		}

		if gitRevision != "" && dockerImageURL != "" {
			return errors.Wrap(err, "cannot use both, git and docker image url")
		}

		builderImage, err := cmd.Flags().GetString("builder-image")
		if err != nil {
			return errors.Wrap(err, "could not read option --builder-image")
		}

		// Syntax:
		// 1. push NAME
		// 2. push NAME PATH
		// 3. push NAME URL --git REV
		// 4. push NAME --docker-image-url URL

		var pathOrUrl string
		if len(args) == 1 {
			if gitRevision != "" {
				// Missing argument is user error. Show usage
				cmd.SilenceUsage = false
				return errors.New("app name or git repository url missing")
			}

			pathOrUrl, err = os.Getwd()
			if err != nil {
				return errors.Wrap(err, "working directory not accessible")
			}
		} else {
			pathOrUrl = args[1]
		}

		if dockerImageURL != "" {
			pathOrUrl = ""
		}

		// Local path is used to look for a manifest file. This is of course
		// nonsense for the push modes taking an url instead of a filesystem path.
		localPath := pathOrUrl

		if gitRevision == "" && dockerImageURL == "" {
			if _, err := os.Stat(pathOrUrl); err != nil {
				// Path issue is user error. Show usage
				cmd.SilenceUsage = false
				return errors.Wrap(err, "path not accessible")
			}
		} else {
			// url mode, no path to look into for a manifest file
			localPath = ""
		}

		ac, err := appConfiguration(cmd, localPath)
		if err != nil {
			return errors.Wrap(err, "unable to get app configuration")
		}

		params := usercmd.PushParams{
			Name:          args[0],
			GitRev:        gitRevision,
			Docker:        dockerImageURL,
			Path:          pathOrUrl,
			BuilderImage:  builderImage,
			Configuration: ac,
		}

		err = client.Push(cmd.Context(), params)
		if err != nil {
			return errors.Wrap(err, "error pushing app to server")
		}

		return nil
	},
}
