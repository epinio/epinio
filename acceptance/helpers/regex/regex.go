package regex

import (
	"fmt"
)

const (
	// DateRegex will check for a date in the '2022-05-19 13:49:20 +0000' UTC format
	DateRegex = "[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} [+][0-9]{4} [A-Z]{3,4}"
)

// TableRow returns a regular expression that will match a line of a CLI table
// The arguments passed should contain the expected text of the cell (or regex, i.e. you can pass '.*' to match anything)
func TableRow(args ...string) string {
	reg := `\|`
	for _, arg := range args {
		reg = fmt.Sprintf(`%s[\s]+%s[\s]+\|`, reg, arg)
	}
	return reg
}
