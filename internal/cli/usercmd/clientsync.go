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
	"github.com/epinio/epinio/internal/version"
	"github.com/pkg/errors"
)

// ClientSync downloads the epinio client binary matching the current OS and
// architecture and replaces the currently running one.
func (c *EpinioClient) ClientSync() error {
	log := c.Log.WithName("Client sync")
	log.Info("start")
	defer log.Info("return")

	v, err := c.API.Info()
	if err != nil {
		return err
	}

	if version.Version == v.Version {
		c.ui.Success().Msgf("Client and server version are the same (%s). Nothing to do!", v.Version)

		return nil
	}

	err = c.Updater.Update(v.Version)
	if err != nil {
		return errors.Wrap(err, "updating the client")
	}

	c.ui.Success().Msgf("Updated epinio client to %s", v.Version)

	return nil
}
