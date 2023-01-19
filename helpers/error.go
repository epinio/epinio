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

package helpers

import "strings"

func Retryable(msg string) bool {
	return strings.Contains(msg, " x509: ") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "Gateway") ||
		strings.Contains(msg, "Configuration Unavailable") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "no endpoints available") ||
		strings.Contains(msg, "x509: certificate signed by unknown authority") ||
		(strings.Contains(msg, "api/v1/namespaces") && strings.Contains(msg, "i/o timeout"))
}

func RetryableCode(code int) bool {
	return code >= 400
}
