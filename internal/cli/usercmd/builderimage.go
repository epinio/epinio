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
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
)

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
