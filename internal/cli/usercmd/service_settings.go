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
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartSettingsShow shows the value of the specified environment variable in
// the named application.
func (c *EpinioClient) ChartSettingsShow(ctx context.Context, settings map[string]models.ChartSetting) {
	log := c.Log.WithName("ChartSettingsShow")
	log.Info("start")
	defer log.Info("return")

	if len(settings) > 0 {
		var keys []string
		for key := range settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		msg := c.ui.Note().WithTable("Key", "Type", "Allowed Values")

		for _, key := range keys {
			spec := settings[key]
			msg = msg.WithTableRow(key, spec.Type, settingToString(spec))
		}

		msg.Msg("Settings")
	} else {
		c.ui.Exclamation().Msg("No settings")
	}
}

func settingToString(spec models.ChartSetting) string {
	// Type expected to be in (string, bool, number, integer)
	if spec.Type == "bool" {
		return ""
	}
	if spec.Type == "map" {
		return ""
	}
	if spec.Type == "string" {
		if len(spec.Enum) > 0 {
			return strings.Join(spec.Enum, ", ")
		}
		return ""
	}
	if spec.Type == "number" || spec.Type == "integer" {
		if len(spec.Enum) > 0 {
			return strings.Join(spec.Enum, ", ")
		}
		if spec.Minimum != "" || spec.Maximum != "" {
			min := "-inf"
			if spec.Minimum != "" {
				min = spec.Minimum
			}
			max := "+inf"
			if spec.Maximum != "" {
				max = spec.Maximum
			}
			return fmt.Sprintf(`[%s ... %s]`, min, max)
		}
		return ""
	}
	return "<unknown type>"
}
