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

package matchers

import (
	"fmt"
	"strings"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

const (
	// dateRegex will check for a date in the '2022-05-19 13:49:20 +0000' UTC format
	dateRegex = "[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} [+][0-9]{4} [A-Z]{3,4}"
)

func HaveATable(args ...types.GomegaMatcher) types.GomegaMatcher {
	return And(args...)
}

func WithHeaders(args ...string) types.GomegaMatcher {
	return And(
		MatchRegexp(tableRow(args...)),
		MatchRegexp(`[|-]*`),
	)
}

func WithRow(args ...string) types.GomegaMatcher {
	return MatchRegexp(tableRow(args...))
}

func WithDate() string {
	return dateRegex
}

// tableRow returns a regular expression that will match a line of a CLI table
// The arguments passed should contain the expected text of the cell (or regex, i.e. you can pass '.*' to match anything)
func tableRow(args ...string) string {
	if len(args) == 0 {
		return ""
	}

	var b strings.Builder
	for _, arg := range args {
		fmt.Fprintf(&b, `[|][\s]+%s[\s]+`, arg)
	}
	b.WriteString(`[|]`)

	return b.String()
}
