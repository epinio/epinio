// Package names collects functions encapsulating the rules for
// constructing a variety of kube resource names
package names

import (
	"crypto/md5" // nolint:gosec // Non-crypto use
	"encoding/hex"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
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

// GenerateResourceName joins the input strings with dots (".")  and
// returns the result, suitably truncated to the maximum length of
// kube resource names.
func GenerateResourceName(names ...string) string {
	name := DNSLabelSafe(strings.Join(names, "-"))
	return TruncateMD5(name, 63)
}

// ReleaseName returns the name of a helm release derived from the
// base string.
func ReleaseName(base string) string {
	name := DNSLabelSafe(base)
	return TruncateMD5(name, 53)
}

// GenerateDNS1123SubDomainName joins the input strings with dots (".")
// and returns the result, suitably truncated to the maximum length
// allowed for the domain names.
func GenerateDNS1123SubDomainName(names ...string) string {
	name := DNSLabelSafe(strings.Join(names, "-"))
	return TruncateMD5(name, validation.DNS1123SubdomainMaxLength)
}

// TruncateMD5 truncates the input string s to the maxLen, if
// necessary. Shorter strings are passed through unchanged. Truncation
// is done by computing an MD5 hash over the __entire__ string and
// then combining it with the (maxLen-32)-sized prefix of the
// input. This result in a string of exactly maxLen characters.  The
// magic 32 is the length of the 16 byte MD5 hash in hex characters.
func TruncateMD5(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	sumHex := md5.Sum([]byte(s)) // nolint:gosec // Non-crypto use
	sum := hex.EncodeToString(sumHex[:])
	suffix := "-" + sum
	suffixLen := len(suffix)

	front := maxLen - suffixLen

	return s[:front] + suffix
}
