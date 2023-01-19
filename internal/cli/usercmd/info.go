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
)

// Info displays information about environment
func (c *EpinioClient) Info() error {
	log := c.Log.WithName("Info")
	log.Info("start")
	defer log.Info("return")

	v, err := c.API.Info()
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Platform", v.Platform).
		WithStringValue("Kubernetes Version", v.KubeVersion).
		WithStringValue("Epinio Server Version", v.Version).
		WithStringValue("Epinio Client Version", version.Version).
		Msg("Epinio Environment")

	return nil
}
