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
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/kyokomi/emoji"
)

// This file implements the SpinProgress form of the Progress
// interface.  This encapsulates the currently used progress
// indicator, a spinner.

type SpinProgress struct {
	s *spinner.Spinner
}

// Standard values for the current spinner-based progress
const spinSet = 37
const spinTime = 100 * time.Millisecond

func NewSpinProgress(message string) *SpinProgress {
	message = emoji.Sprint(message + " :zzz:")
	s := &SpinProgress{
		s: spinner.New(spinner.CharSets[spinSet], spinTime),
	}
	s.s.Suffix = message
	s.s.Start()
	return s
}

func (p *SpinProgress) Start() {
	p.s.Start()
}

func (p *SpinProgress) Stop() {
	p.s.Stop()
}

// ChangeMessagef extends the spinner-based progress with the ability
// to change the message mid-flight
func (p *SpinProgress) ChangeMessagef(message string, a ...interface{}) {
	p.ChangeMessage(fmt.Sprintf(message, a...))
}

// ChangeMessage extends the spinner-based progress with the ability
// to change the message mid-flight
func (p *SpinProgress) ChangeMessage(message string) {
	message = emoji.Sprint(message + " :zzz:")
	p.s.Stop()
	p.s.Suffix = message
	p.s.Start()
}
