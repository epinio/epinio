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

package testenv

import "path"

var root = ".."

func SetRoot(dir string) {
	root = dir
}

func Root() string {
	return root
}

// AssetPath returns the path to an asset
func AssetPath(p ...string) string {
	parts := append([]string{root, "assets"}, p...)
	return path.Join(parts...)
}

// TestAssetPath returns the relative path to the test assets
func TestAssetPath(file string) string {
	return path.Join(root, "assets", "tests", file)
}
