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
	"fmt"
	"strings"
)

// EnumValue implements the Value interface
// It can be used to define a flag with a set of allowed values
// Ref:
// - https://github.com/spf13/pflag/blob/2e9d26c8c37aae03e3f9d4e90b7116f5accb7cab/flag.go#L185-L191
// - https://github.com/spf13/pflag/issues/236#issuecomment-931600452
type EnumValue struct {
	Allowed []string
	Value   string
}

// NewEnumValue give a list of allowed flag parameters, where the second argument is the default
func NewEnumValue(allowed []string, d string) *EnumValue {
	return &EnumValue{
		Allowed: allowed,
		Value:   d,
	}
}

func (a EnumValue) String() string {
	return a.Value
}

func (a *EnumValue) Set(p string) error {
	isIncluded := func(opts []string, val string) bool {
		for _, opt := range opts {
			if val == opt {
				return true
			}
		}
		return false
	}
	if !isIncluded(a.Allowed, p) {
		return fmt.Errorf("%s is not included in %s", p, strings.Join(a.Allowed, ","))
	}
	a.Value = p
	return nil
}

func (a *EnumValue) Type() string {
	return "string"
}
