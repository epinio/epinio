package termui

// WithTable prints a new table
func (u *Message) WithTable(headers ...string) *Message {
	u.tableHeaders = append(u.tableHeaders, headers)
	u.tableData = append(u.tableData, [][]string{})
	return u
}

// WithTableRow adds a row in the latest table
func (u *Message) WithTableRow(values ...string) *Message {
	if len(u.tableHeaders) < 1 {
		return u.WithTable(make([]string, len(values))...).WithTableRow(values...)
	}

	u.tableData[len(u.tableData)-1] = append(u.tableData[len(u.tableData)-1], values)

	return u
}
