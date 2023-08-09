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
)

// ExportregistryList displays a table of all known export registries.
func (c *EpinioClient) ExportregistryList(ctx context.Context) error {
	log := c.Log.WithName("ExportregistryList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		Msg("Show export registries")

	exportregistries, err := c.API.ExportregistryList()
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Name", "URL")

	for _, exportregistry := range exportregistries {
		name := exportregistry.Name
		url := exportregistry.URL
		msg = msg.WithTableRow(name, url)
	}

	msg.Msg("Ok")
	return nil
}

// ExportregistryMatching retrieves all export registries in the cluster, for the given prefix
func (c *EpinioClient) ExportregistryMatching(prefix string) []string {
	log := c.Log.WithName("ExportregistryMatching")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.ExportregistryMatch(prefix)
	if err != nil {
		log.Error(err, "calling exportregistry match endpoint")
		return []string{}
	}

	return resp.Names
}
