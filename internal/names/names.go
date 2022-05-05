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

// DNSLabelSafe filters invalid characters and returns a string that is safe to use as a DNS label.
// It does not enforce the required string length, see `Sanitize`.
//
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
func DNSLabelSafe(name string) string {
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

// GenerateResourceNameTruncated joins the input strings with dashes("-")
// and returns the checksum of the produced string after removing
// any characters that are invalid for kubernetes resource names
// and prefixing the checksum with up to (maxLen - Sha1sumLength) characters of the original
// string. It concatenates the prefix with the checksum with a "-".
// This way the generated name:
// - is always valid for a resource name
// - is never longer than maxLen characters
// - has low probability of collisions
// Since the checksum must always be included, this function shouldn't be used
// to produce names shorter than Sha1sumLength characters.
func GenerateResourceNameTruncated(originalName string, maxLen int) string {
	sumArray := sha1.Sum([]byte(originalName)) // nolint:gosec // Non-crypto use
	sum := hex.EncodeToString(sumArray[:])

	if maxLen < Sha1sumLength {
		panic(fmt.Sprintf("shouldn't try to generate a resource name shorter than %d characters", Sha1sumLength))
	}

	// Don't prefix anything if we don't have enough room for at least a
	// letter from the originalName plus the dash "-" to separate it from the checksum
	if maxLen < 42 {
		return sum
	}

	safePrefix := Truncate(DNSLabelSafe(originalName), (maxLen - (Sha1sumLength + 1)))

	return fmt.Sprintf("%s-%s", safePrefix, sum)
}

// ReleaseName returns the name of a helm release derived from the base string.
func ReleaseName(base string) string {
	return GenerateResourceNameTruncated(base, 53)
}

// Truncate truncates the input string s to the maxLen, if
// necessary. Shorter strings are passed through unchanged.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}
