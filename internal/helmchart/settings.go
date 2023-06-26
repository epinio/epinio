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

package helmchart

import (
	"errors"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SettingsToChart converts from an unstructured representation of the CR to the internal
// structure.
func SettingsToChart(chart *unstructured.Unstructured) (map[string]models.ChartSetting, error) {
	theSettings, _, err := unstructured.NestedMap(chart.UnstructuredContent(), "spec", "settings")
	if err != nil {
		return nil, errors.New("spec settings should be map")
	}
	settings := make(map[string]models.ChartSetting)
	for key := range theSettings {
		fieldType, _, err := unstructured.NestedString(theSettings, key, "type")
		if err != nil {
			return nil, errors.New("settings type should be string")
		}
		fieldMin, _, err := unstructured.NestedString(theSettings, key, "minimum")
		if err != nil {
			return nil, errors.New("settings minimum should be string")
		}
		fieldMax, _, err := unstructured.NestedString(theSettings, key, "maximum")
		if err != nil {
			return nil, errors.New("settings maximum should be string")
		}
		fieldEnum, _, err := unstructured.NestedStringSlice(theSettings, key, "enum")
		if err != nil {
			return nil, errors.New("settings enum should be string slice")
		}

		settings[key] = models.ChartSetting{
			Type:    fieldType,
			Minimum: fieldMin,
			Maximum: fieldMax,
			Enum:    fieldEnum,
		}
	}

	return settings, nil
}
