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
	"sync"
	"time"

	"github.com/fatih/color"
)

// This file implements the DotProgress form of the Progress
// interface.  This encapsulates the new indicator to use.
// It prints a series of dots as time goes by.

type DotProgress struct {
	ui       *UI
	mu       *sync.RWMutex //
	Delay    time.Duration // Delay is the speed of the indicator
	active   bool          // active holds the state of the spinner
	stopChan chan struct{} // stopChan is a channel used to stop the indicator
}

// Standard values for the current dot-based progress
const dotTime = 1 * time.Second

func NewDotProgress(ui *UI, message string) *DotProgress {
	message = mfinal(message)
	p := &DotProgress{
		ui:       ui,
		Delay:    dotTime,
		mu:       &sync.RWMutex{},
		active:   false,
		stopChan: make(chan struct{}, 1),
	}
	p.ui.ProgressNote().V(1).KeepLine().Msg(message)
	p.Start()
	return p
}

func (p *DotProgress) Start() {
	p.mu.Lock()
	if p.active {
		p.mu.Unlock()
		return
	}
	p.active = true
	p.mu.Unlock()

	go func() {
		for {
			select {
			case <-p.stopChan:
				return
			default:
				p.mu.Lock()
				if !p.active {
					p.mu.Unlock()
					return
				}
				p.ui.Normal().
					Compact().
					KeepLine().
					Msg(color.MagentaString("."))
				delay := p.Delay
				p.mu.Unlock()
				time.Sleep(delay)
			}
		}
	}()
}

func (p *DotProgress) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active {
		p.active = false
		p.ui.Normal().V(1).Compact().Msg("")
		p.stopChan <- struct{}{}
	}
}

// ChangeMessagef extends the dot-based progress with the ability to
// change the message mid-flight
func (p *DotProgress) ChangeMessagef(message string, a ...interface{}) {
	p.ChangeMessage(fmt.Sprintf(message, a...))
}

// ChangeMessage extends the dot-based progress with the ability to
// change the message mid-flight
func (p *DotProgress) ChangeMessage(message string) {
	// Prevent the restart dance if the message would not be shown
	// anyway, due to a low verbosity level. This keeps the dots
	// nicer.
	if p.ui.verbosity < 1 {
		return
	}

	message = mfinal(message)
	p.Stop()
	p.ui.ProgressNote().V(1).KeepLine().Msg(message)
	p.Start()
}

func mfinal(message string) string {
	return message + " "
}
