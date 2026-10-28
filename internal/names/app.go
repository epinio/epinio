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

package names

import "strings"

// reservedAppNames collide with static path segments under
// /namespaces/:namespace/applications/<segment> (for example the
// undeployed-apps list at .../applications/undeployed). Creating an app
// with one of these names would make AppShow unreachable for that app.
var reservedAppNames = map[string]struct{}{
	"undeployed": {},
}

// IsReservedAppName reports whether name is reserved for an API route and
// must not be used as an application name.
func IsReservedAppName(name string) bool {
	_, ok := reservedAppNames[strings.ToLower(name)]
	return ok
}
