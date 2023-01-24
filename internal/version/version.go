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

// Package version contains the variable holding the client's version number.
package version

// Version contains the client's version number. The string found in
// the code is just a placeholder for the actual value inserted when
// building the client. See the LDFLAGS variable in the Makefile.
var Version = "v0.0.0-dev"

// ChartVersion contains the version of the Helm Chart used to release Epinio.
// It's the Product Release version, and it matches the Epinio Github Release.
var ChartVersion = "v0.0.0-dev"
