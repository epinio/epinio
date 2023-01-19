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

package logprinter

import (
	"encoding/json"
	"text/template"

	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// DefaultSingleNamespaceTemplate returns a printing template used when
// printing with colors and watching resources in a single namespace
func DefaultSingleNamespaceTemplate() *template.Template {
	t := "[{{ color .PodColor .PodName}}] {{color .ContainerColor .ContainerName}} {{.Message}}"

	funs := map[string]interface{}{
		"json": func(in interface{}) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	template, err := template.New("log").Funcs(funs).Parse(t)
	if err != nil {
		panic(errors.Wrap(err, "unable to parse template"))
	}

	return template
}
