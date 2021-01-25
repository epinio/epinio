package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/olekukonko/tablewriter"
)

type msgType int
type valueType int
type valueVariant int

const (
	normal msgType = iota
	exclamation
	problem
	note
	success
	progress
)

const (
	tBool valueType = iota
	tString
	tInt
)

const (
	show valueVariant = iota
	ask
)

// UI contains functionality for dealing with the user
// on the CLI
type UI struct {
}

// Message represents a piece of information we want displayed to the user
type Message struct {
	msgType      msgType
	end          int
	interactions []interaction
	tableHeaders [][]string
	tableData    [][][]string
}

type interaction struct {
	variant   valueVariant
	valueType valueType
	name      string
	value     interface{}
}

// Progress abstracts the operations for a progress meter and/or
// spinner used to indicate the cli waiting for some background
// operation to complete.
type Progress interface {
	Start()
	Stop()
	ChangeMessage(message string)
	ChangeMessagef(message string, a ...interface{})
}

// NewUI creates a new UI
func NewUI() *UI {
	return &UI{}
}

// Progress creates, configures, and returns an active progress
// meter. It accepts a formatted message.
func (u *UI) Progressf(message string, a ...interface{}) Progress {
	return u.Progress(fmt.Sprintf(message, a...))
}

// Progress creates, configures, and returns an active progress
// meter. It accepts a fixed message.
func (u *UI) Progress(message string) Progress {
	return NewDotProgress(u, message)
	// return NewSpinProgress(message)
}

// Normal returns a UIMessage that prints a normal message
func (u *UI) Normal() *Message {
	return &Message{
		msgType:      normal,
		interactions: []interaction{},
		end:          -1,
	}
}

// Exclamation returns a UIMessage that prints an exclamation message
func (u *UI) Exclamation() *Message {
	return &Message{
		msgType:      exclamation,
		interactions: []interaction{},
		end:          -1,
	}
}

// Note returns a UIMessage that prints a note message
func (u *UI) Note() *Message {
	return &Message{
		msgType:      note,
		interactions: []interaction{},
		end:          -1,
	}
}

// Success returns a UIMessage that prints a success message
func (u *UI) Success() *Message {
	return &Message{
		msgType:      success,
		interactions: []interaction{},
		end:          -1,
	}
}

// ProgressNote returns a UIMessage that prints a progress-related message
func (u *UI) ProgressNote() *Message {
	return &Message{
		msgType:      progress,
		interactions: []interaction{},
		end:          -1,
	}
}

// Problem returns a Message that prints a message that describes a problem
func (u *UI) Problem() *Message {
	return &Message{
		msgType:      problem,
		interactions: []interaction{},
		end:          -1,
	}
}

func (u *UI) Raw(message string) {
	fmt.Printf("%s", message)
}

// Msgf prints a formatted message on the CLI
func (u *Message) Msgf(message string, a ...interface{}) {
	u.Msg(fmt.Sprintf(message, a...))
}

// Msg prints a message on the CLI, resolving emoji as it goes
func (u *Message) Msg(message string) {
	message = emoji.Sprint(message)

	// Always print a newline before starting output
	if message != "" {
		fmt.Println()
	}

	switch u.msgType {
	case normal:
		fmt.Println(message)
	case exclamation:
		message = emoji.Sprintf(":warning: %s", message)
		color.Yellow(message)
	case note:
		message = emoji.Sprintf(":ship:%s", message)
		color.Blue(message)
	case success:
		message = emoji.Sprintf(":heavy_check_mark: %s", message)
		color.Green(message)
	case progress:
		message = emoji.Sprintf(":three-thirty: %s", message)
		fmt.Println(message)
	case problem:
		message = emoji.Sprintf(":forbidden:%s", message)
		color.Red(message)
	}

	for _, interaction := range u.interactions {
		switch interaction.variant {
		case ask:
			fmt.Printf("> ")
			switch interaction.valueType {
			case tBool:
				interaction.value = readBool()
			case tInt:
				interaction.value = readInt()
			case tString:
				interaction.value = readString()
			}
		case show:
			switch interaction.valueType {
			case tBool:
				fmt.Printf("%s: %s\n", emoji.Sprint(interaction.name), color.MagentaString("%b", interaction.value))
			case tInt:
				fmt.Printf("%s: %s\n", emoji.Sprint(interaction.name), color.CyanString("%d", interaction.value))
			case tString:
				fmt.Printf("%s: %s\n", emoji.Sprint(interaction.name), color.GreenString("%s", interaction.value))
			}
		}
	}

	for idx, headers := range u.tableHeaders {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(headers)
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")

		if idx < len(u.tableData) {
			table.AppendBulk(u.tableData[idx])
		}

		table.Render()
	}

	if u.end > -1 {
		os.Exit(u.end)
	}
}

// WithEnd ends the app
func (u *Message) WithEnd(code int) *Message {
	u.end = code
	return u
}

// WithBoolValue adds a bool value to be printed in the message
func (u *Message) WithBoolValue(name string, value bool) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tBool,
		value:     value,
	})
	return u
}

// WithStringValue adds a string value to be printed in the message
func (u *Message) WithStringValue(name string, value string) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tString,
		value:     value,
	})
	return u
}

// WithIntValue adds an int value to be printed in the message
func (u *Message) WithIntValue(name string, value int) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tInt,
		value:     value,
	})
	return u
}

// WithAskBool waits for the user's input for a boolean value
func (u *Message) WithAskBool(name string, result *bool) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tBool,
		value:     result,
	})
	return u
}

// WithAskString waits for the user's input for a string value
func (u *Message) WithAskString(name string, result *string) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tString,
		value:     result,
	})
	return u
}

// WithAskInt waits for the user's input for an int value
func (u *Message) WithAskInt(name string, result *int) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   show,
		valueType: tInt,
		value:     result,
	})
	return u
}

func readBool() bool {
	var value bool
	fmt.Scanf("%b", &value)

	return value
}

func readString() string {
	var value string
	fmt.Scanf("%s", &value)

	return value
}

func readInt() int {
	var value int
	fmt.Scanf("%d", &value)

	return value
}
