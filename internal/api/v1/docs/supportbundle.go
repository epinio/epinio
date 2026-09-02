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

package docs

//go:generate swagger generate spec

// swagger:route GET /support-bundle support SupportBundle
// Collect logs and diagnostics from the Epinio components and return them as a
// tar archive. Admin only.
// responses:
//   200: SupportBundleResponse

// swagger:parameters SupportBundle
type SupportBundleParam struct {
	// Include application logs as well (true/false, default false).
	// Also accepted as "include_app_logs".
	// in: query
	IncludeApps string `json:"include_apps"`
	// Lines to tail from each component (default 1000, maximum 10000)
	// in: query
	Tail string `json:"tail"`
}

// swagger:response SupportBundleResponse
type SupportBundleResponse struct {
	// in: body
	Body []byte
}
