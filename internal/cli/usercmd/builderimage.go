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

package usercmd

import (
	"context"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// BuilderImageList displays a table of all known builder images.
func (c *EpinioClient) BuilderImageList(ctx context.Context) error {
	log := c.Log.WithName("BuilderImageList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		Msg("Show Builder Images")

	images, err := c.API.BuilderImageList()
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Default", "Name", "Image", "Short Description", "Bound Apps")

	for _, image := range images {
		mark := ""
		if image.Default {
			mark = color.BlueString("*")
		}
		boundApps := ""
		if image.BoundApps {
			boundApps = "yes"
		}
		msg = msg.WithTableRow(mark, image.Meta.Name, image.Image, image.ShortDescription, boundApps)
	}

	msg.Msg("Ok")
	return nil
}

// BuilderImageShow displays the details of the named builder image.
func (c *EpinioClient) BuilderImageShow(ctx context.Context, name string) error {
	log := c.Log.WithName("BuilderImageShow")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Show builder image details")

	image, err := c.API.BuilderImageShow(name)
	if err != nil {
		return err
	}

	defaultMark := "no"
	if image.Default {
		defaultMark = "yes"
	}
	boundApps := "no"
	if image.BoundApps {
		boundApps = "yes"
	}

	c.ui.Note().WithTable("Key", "Value").
		WithTableRow("Name", image.Meta.Name).
		WithTableRow("Created", formatCreatedAt(image.Meta.CreatedAt)).
		WithTableRow("Image", image.Image).
		WithTableRow("Short", image.ShortDescription).
		WithTableRow("Description", image.Description).
		WithTableRow("Default", defaultMark).
		WithTableRow("Bound Apps", boundApps).
		Msg("Details:")

	c.ui.Success().Msg("Ok")

	return nil
}

// BuilderImageMatching retrieves all builder images in the cluster, for the given prefix
func (c *EpinioClient) BuilderImageMatching(prefix string) []string {
	log := c.Log.WithName("BuilderImageMatching")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.BuilderImageMatch(prefix)
	if err != nil {
		log.Error(err, "calling builder image match endpoint")
		return []string{}
	}

	return resp.Names
}

// BuilderImageCreate creates a builder image from the supplied request.
func (c *EpinioClient) BuilderImageCreate(request models.BuilderImageCreateRequest) error {
	log := c.Log.WithName("BuilderImageCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", request.Name).
		WithStringValue("Image", request.Image).
		Msg("Creating Builder Image...")

	_, err := c.API.BuilderImageCreate(request)
	if err != nil {
		return errors.Wrap(err, "builder image create failed")
	}

	c.ui.Success().
		WithStringValue("Name", request.Name).
		Msg("Builder Image Created.")

	return nil
}

// BuilderImageUpdate updates the named builder image. Omitted request fields
// leave the corresponding values unchanged on the server.
func (c *EpinioClient) BuilderImageUpdate(name string, request models.BuilderImageUpdateRequest) error {
	log := c.Log.WithName("BuilderImageUpdate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Updating Builder Image...")

	_, err := c.API.BuilderImageUpdate(name, request)
	if err != nil {
		return errors.Wrap(err, "builder image update failed")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		Msg("Builder Image Updated.")

	return nil
}

// BuilderImageDelete deletes the named builder image.
func (c *EpinioClient) BuilderImageDelete(name string) error {
	log := c.Log.WithName("BuilderImageDelete")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Deleting Builder Image...")

	_, err := c.API.BuilderImageDelete(name)
	if err != nil {
		return errors.Wrap(err, "builder image delete failed")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		Msg("Builder Image Removed.")

	return nil
}
