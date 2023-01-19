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

// Package names collects functions encapsulating the rules for
// constructing a variety of kube resource names
package names

import (
	// nolint:gosec // Non-crypto use
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const (
	Sha1sumLength = 40 // The length of a sha1sum checksum
)

var allowedDNSLabelChars = regexp.MustCompile("[^-a-z0-9]*")

// DNSLabelSafe filters invalid characters and returns a string that is safe to
// use as Kubernetes resource name.
//
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
func DNSLabelSafe(name string) string {
	name = strings.TrimLeft(name, "0123456789") // leading digits
	name = strings.Replace(name, "_", "-", -1)
	name = strings.ToLower(name)
	name = allowedDNSLabelChars.ReplaceAllLiteralString(name, "")
	name = strings.TrimLeft(name, "-")
	name = strings.TrimRight(name, "-")
	return name
}

func GenerateResourceName(names ...string) string {
	originalName := strings.Join(names, "-")
	return GenerateResourceNameTruncated(originalName, 63)
}

// MD5String compute the hash of the passed value and returns the first 'length' characters
// If the length is -1 or greater than the md5 hash then the whole hash is returned
func MD5String(value string, length int) string {
	sumArray := sha1.Sum([]byte(value)) // nolint:gosec // Non-crypto use
	sum := hex.EncodeToString(sumArray[:])

	if length == -1 || length > len(sum) {
		return sum
	}
	return sum[:length]
}

// GenerateResourceNameTruncated joins the input strings with dashes("-")
// and returns the checksum of the produced string after removing
// any characters that are invalid for kubernetes resource names
// and prefixing the checksum with up to (maxLen - Sha1sumLength) characters of the original
// string. It concatenates the prefix with the checksum with a "-".
// This way the generated name:
// - is always valid for a resource name
// - is never longer than maxLen characters
// - has low probability of collisions
// NOTE: Since the checksum must always be included, this function shouldn't be used
// to produce names shorter than Sha1sumLength characters.
func GenerateResourceNameTruncated(originalName string, maxLen int) string {
	sum := MD5String(originalName, -1)

	// We allow maxLen less than the sha hash. We take the prefix of the hash in that
	// case.  While there is some risk of conflict it should be tolerable until we
	// reach maxLen < 10 or so.

	// Don't prefix anything if we don't have enough room for at least a
	// letter from the originalName plus the dash "-" to separate it from the checksum
	if maxLen < 42 {
		return fmt.Sprintf("x%s", sum[1:maxLen-1])
	}

	safePrefix := Truncate(DNSLabelSafe(originalName), (maxLen - (Sha1sumLength + 1)))
	if len(safePrefix) > 0 {
		safePrefix = safePrefix + "-" // Split the prefix with a dash
	}

	return fmt.Sprintf("%s%s", safePrefix, sum)
}

// ReleaseName returns the name of a helm release derived from the base string.
func ReleaseName(base string) string {
	return GenerateResourceNameTruncated(base, 53)
}

// ServiceReleaseName returns the name of a helm release derived from the base string.
func ServiceReleaseName(base string) string {
	// The integral helm client deploying the chart generates derived names for secrets and pods
	// from the name of the chart, and __does not__ length limit them properly.  As one of the
	// components is the name of the chart we cannot fully account for it here (*). We keep 33
	// under the limit for suitable space.  (*) NOTE: While some places have the chart name
	// available, others do not.
	return GenerateResourceNameTruncated(base, 30)
}

// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
func ServiceHelmChartName(name, namespace string) string {
	// The helm controller deploying the chart generates derived names for secrets and
	// pods from the name of the chart, and __does not__ length limit them properly.
	// As one of the components is the name of the chart we cannot fully account for
	// it here (*). We keep 33 under the limit for suitable space.
	// (*) NOTE: While some places have the chart name available, others do not.
	return GenerateResourceNameTruncated(fmt.Sprintf("%s-%s", namespace, name), 30)
}

// Truncate truncates the input string s to the maxLen, if
// necessary. Shorter strings are passed through unchanged.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}
