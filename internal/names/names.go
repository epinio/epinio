// Package names collects functions encapsulating the rules for
// constructing a variety of kube resource names
package names

import (
	"crypto/md5" // nolint:gosec // Non-crypto use
	"encoding/hex"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// ServiceName returns the name of a kube service derived from the
// base string. It ensures that things like leading digits are
// sufficiently hidden to prevent kube from erroring out on the name.
func ServiceName(base string) string {
	return GenerateResourceName("s-" + base)
}

// IngressName returns the name of a kube ingress derived from the
// base string. It ensures that things like leading digits are
// sufficiently hidden to prevent kube from erroring out on the name.
// It also replaces "/" characters with "-" to produce a valid resource
// name from a route (which may have "/" in it).
func IngressName(base string) string {
	baseSafe := strings.ReplaceAll(base, "/", "-")
	return GenerateResourceName("i-" + baseSafe)
}

// GenerateResourceName joins the input strings with dots (".")  and
// returns the result, suitably truncated to the maximum length of
// kube resource names.
func GenerateResourceName(names ...string) string {
	return TruncateMD5(strings.Join(names, "."), 63)
}

// GenerateDNS1123SubDomainName joins the input strings with dots (".")
// and returns the result, suitably truncated to the maximum length
// allowed for the domain names.
func GenerateDNS1123SubDomainName(names ...string) string {
	return TruncateMD5(strings.Join(names, "."), validation.DNS1123SubdomainMaxLength)
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
