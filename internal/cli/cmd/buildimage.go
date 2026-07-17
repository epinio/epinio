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
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewBuildImageCmd returns a new 'epinio buildimage' command
func NewBuildImageCmd(client *usercmd.EpinioClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildimage",
		Short: "Epinio builder image management",
		Long:  `Manage epinio builder images`,
	}

	cmd.AddCommand(
		NewBuildImageCreateCmd(client),
		NewBuildImageUpdateCmd(client),
		NewBuildImageDeleteCmd(client),
	)

	return cmd
}

// BuildImageWriteConfig holds the flags shared by the create and update
// commands. The update command reuses the same set minus --name.
type BuildImageWriteConfig struct {
	name             string
	image            string
	description      string
	shortDescription string
}

// buildImageWriteFlags registers the flags common to create and update (i.e.
// every write flag except --name, which only create carries).
func buildImageWriteFlags(cmd *cobra.Command, cfg *BuildImageWriteConfig) {
	cmd.Flags().StringVar(&cfg.image, "image", "", "full image reference")
	cmd.Flags().StringVar(&cfg.description, "description", "", "long description")
	cmd.Flags().StringVar(&cfg.shortDescription, "short-description", "", "short description")
}

// NewBuildImageCreateCmd returns a new 'epinio buildimage create' command
func NewBuildImageCreateCmd(client *usercmd.EpinioClient) *cobra.Command {
	cfg := BuildImageWriteConfig{}
	cmd := &cobra.Command{
		Use:   "create --name NAME --image IMAGE [flags]",
		Short: "Create a builder image",
		Long:  "Create a builder image",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.name == "" {
				return errors.New("--name is required")
			}
			if cfg.image == "" {
				return errors.New("--image is required")
			}

			request := models.BuilderImageCreateRequest{
				Name:             cfg.name,
				Image:            cfg.image,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
			}

			return errors.Wrap(client.BuilderImageCreate(request), "error creating builder image")
		},
	}

	cmd.Flags().StringVar(&cfg.name, "name", "", "builder image name (required)")
	buildImageWriteFlags(cmd, &cfg)

	return cmd
}

// NewBuildImageUpdateCmd returns a new 'epinio buildimage update' command
func NewBuildImageUpdateCmd(client *usercmd.EpinioClient) *cobra.Command {
	cfg := BuildImageWriteConfig{}
	cmd := &cobra.Command{
		Use:   "update NAME [flags]",
		Short: "Update a builder image",
		Long:  "Update a builder image. Unset flags leave the corresponding fields unchanged.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			request := models.BuilderImageUpdateRequest{
				Image:            cfg.image,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
			}

			return errors.Wrap(client.BuilderImageUpdate(args[0], request), "error updating builder image")
		},
	}

	buildImageWriteFlags(cmd, &cfg)

	return cmd
}

// NewBuildImageDeleteCmd returns a new 'epinio buildimage delete' command
func NewBuildImageDeleteCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a builder image",
		Long:  "Delete a builder image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.Wrap(client.BuilderImageDelete(args[0]), "error deleting builder image")
		},
	}
}
