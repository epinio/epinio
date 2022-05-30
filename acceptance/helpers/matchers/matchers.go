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
