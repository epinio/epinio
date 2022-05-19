package regex

import "fmt"

// TableRow returns a regular expression that will match a line of a CLI table
// The arguments passed should contain the expected text of the cell (or regex, i.e. you can pass '.*' to match anything)
func TableRow(args ...string) string {
	reg := `\|`
	for _, arg := range args {
		reg = fmt.Sprintf(`%s[\s]+%s[\s]+\|`, reg, arg)
	}
	return reg
}
