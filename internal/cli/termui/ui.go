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

package termui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/viper"
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
	question
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
	output      io.Writer
	verbosity   int // Verbosity level for user messages.
	jsonEnabled bool
}

// Message represents a piece of information we want displayed to the user
type Message struct {
	ui           *UI // For access to requested verbosity.
	level        int
	msgType      msgType
	end          int
	compact      bool
	keepline     bool
	wait         time.Duration
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
	return &UI{
		output:    color.Output,
		verbosity: verbosity(),
	}
}

func (u *UI) EnableJSON() {
	u.verbosity = -1
	u.jsonEnabled = true
}

func (u *UI) DisableJSON() {
	u.verbosity = verbosity()
	u.jsonEnabled = false
}

func (u *UI) JSON(value any) error {
	if u.jsonEnabled {
		return json.NewEncoder(u.output).Encode(value)
	}
	return nil
}

func (u *UI) JSONEnabled() bool {
	return u.jsonEnabled
}

func (u *UI) Raw(message string) {
	fmt.Fprintf(u.output, "%s", message)
}

func (u *UI) SetOutput(output io.Writer) {
	if output == nil {
		output = color.Output
	}
	u.output = output
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
		ui:           u,
		msgType:      normal,
		interactions: []interaction{},
		end:          -1,
	}
}

// Exclamation returns a UIMessage that prints an exclamation message
func (u *UI) Exclamation() *Message {
	return &Message{
		ui:           u,
		msgType:      exclamation,
		interactions: []interaction{},
		end:          -1,
	}
}

// Question returns a UIMessage that prints a question. Best used with `WithAsk...` modifiers.
func (u *UI) Question() *Message {
	return &Message{
		ui:           u,
		msgType:      question,
		interactions: []interaction{},
		end:          -1,
	}
}

// Note returns a UIMessage that prints a note message
func (u *UI) Note() *Message {
	return &Message{
		ui:           u,
		msgType:      note,
		interactions: []interaction{},
		end:          -1,
	}
}

// Success returns a UIMessage that prints a success message
func (u *UI) Success() *Message {
	return &Message{
		ui:           u,
		msgType:      success,
		interactions: []interaction{},
		end:          -1,
	}
}

// ProgressNote returns a UIMessage that prints a progress-related message
func (u *UI) ProgressNote() *Message {
	return &Message{
		ui:           u,
		msgType:      progress,
		interactions: []interaction{},
		end:          -1,
	}
}

// Problem returns a Message that prints a message that describes a problem
func (u *UI) Problem() *Message {
	return &Message{
		ui:           u,
		msgType:      problem,
		interactions: []interaction{},
		end:          -1,
	}
}

// Msgf prints a formatted message on the CLI
func (u *Message) Msgf(message string, a ...interface{}) {
	u.Msg(fmt.Sprintf(message, a...))
}

// Msg prints a message on the CLI, resolving emoji as it goes
func (u *Message) Msg(message string) {
	// Ignore messages higher than the requested verbosity.
	if u.level > u.ui.verbosity {
		return
	}

	message = emoji.Sprint(message)

	// Print a newline before starting output, if not compact.
	if message != "" && !u.compact {
		fmt.Println()
	}

	if !u.keepline {
		message += "\n"
	}

	switch u.msgType {
	case question:
		message = emoji.Sprintf(":question: %s", message)
		message = color.RedString(message)
	case normal:
	case exclamation:
		message = emoji.Sprintf(":warning: %s", message)
		message = color.YellowString(message)
	case note:
		message = emoji.Sprintf(":ship: %s", message)
		message = color.BlueString(message)
	case success:
		message = emoji.Sprintf(":heavy_check_mark: %s", message)
		message = color.GreenString(message)
	case progress:
		message = emoji.Sprintf(":three-thirty: %s", message)
	case problem:
		message = emoji.Sprintf(":cross_mark: %s", message)
		message = color.RedString(message)
	}

	fmt.Fprintf(u.ui.output, "%s", message)

	for _, interaction := range u.interactions {
		switch interaction.variant {
		case ask:
			fmt.Printf("> ")
			switch interaction.valueType {
			case tBool:
				b, _ := interaction.value.(*bool)
				*b = readBool()
			case tInt:
				i, _ := interaction.value.(*int)
				*i = readInt()
			case tString:
				s, _ := interaction.value.(*string)
				*s = readString()
			}
		case show:
			switch interaction.valueType {
			case tBool:
				fmt.Fprintf(u.ui.output, "%s: %s\n", emoji.Sprint(interaction.name), color.MagentaString("%t", interaction.value))
			case tInt:
				fmt.Fprintf(u.ui.output, "%s: %s\n", emoji.Sprint(interaction.name), color.CyanString("%d", interaction.value))
			case tString:
				fmt.Fprintf(u.ui.output, "%s: %s\n", emoji.Sprint(interaction.name), color.GreenString("%s", interaction.value))
			}
		}
	}

	for idx, headers := range u.tableHeaders {
		table := tablewriter.NewWriter(u.ui.output)
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

	if u.wait > 0 {
		timeoutChan := time.After(u.wait)
		doneChan := make(chan struct{}, 1)

		go func(done chan struct{}) {
			reader := bufio.NewReader(os.Stdin)
			_, _ = reader.ReadString('\n')
			done <- struct{}{}
		}(doneChan)

		// Now just wait for timeout or input
		select {
		case <-timeoutChan:
		case <-doneChan:
		}
	}
}

// Timeout sets a timeout for the message to wait after printing
// before continuing. This disables any previous `WithEnd`.
func (u *Message) Timeout(wait time.Duration) *Message {
	u.wait = wait
	u.end = -1
	return u
}

// V incrementally modifies the message level.
func (u *Message) V(delta int) *Message {
	u.level += delta
	return u
}

// KeepLine disables the printing of a newline after a message output
func (u *Message) KeepLine() *Message {
	u.keepline = true
	return u
}

// KeeplineUnder disables the printing of a newline after a message
// output, if the verbosity level is below the specified.
func (u *Message) KeeplineUnder(level int) *Message {
	if u.ui.verbosity < level {
		u.keepline = true
	}
	return u
}

// Compact disables the printing of a newline before starting output
func (u *Message) Compact() *Message {
	u.compact = true
	return u
}

// WithEnd ends the entire process after printing the message.  This
// also disables any previous `Timeout`.
func (u *Message) WithEnd(code int) *Message {
	u.end = code
	u.wait = 0
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
		variant:   ask,
		valueType: tInt,
		value:     value,
	})
	return u
}

// WithAskBool waits for the user's input for a boolean value
func (u *Message) WithAskBool(name string, result *bool) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   ask,
		valueType: tBool,
		value:     result,
	})
	return u
}

// WithAskString waits for the user's input for a string value
func (u *Message) WithAskString(name string, result *string) *Message {
	u.interactions = append(u.interactions, interaction{
		name:      name,
		variant:   ask,
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
	value = strings.TrimSpace(value)

	return value
}

func readInt() int {
	var value int
	fmt.Scanf("%d", &value)

	return value
}

// verbosity returns the verbosity argument
func verbosity() int {
	return viper.GetInt("verbosity")
}
