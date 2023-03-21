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

package usercmd

import "time"

func (c *EpinioClient) trackDeletion(names []string, poller func() []string) {

	// set of tracked names
	tracked := map[string]struct{}{}
	for _, name := range names {
		tracked[name] = struct{}{}
	}

	for {
		// poll to see the existing names
		time.Sleep(2 * time.Second)

		current := map[string]struct{}{}
		for _, name := range poller() {
			current[name] = struct{}{}
		}

		// check which of the tracked names do not exist any longer, and report them.

		for name := range tracked {
			if _, found := current[name]; !found {
				delete(tracked, name)
				c.ui.Success().Msgf("%s deleted", name)
			}
		}

		// End when there is nothing to track any more. Or the end of the process.

		if len(tracked) == 0 {
			break
		}
	}
}
