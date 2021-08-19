package names

import (
	"crypto/md5"
	"encoding/hex"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

func ServiceName(base string) string {
	return GenerateResourceName("s-" + base)
}

func IngressName(base string) string {
	return GenerateResourceName("i-" + base)
}

func GenerateResourceName(names ...string) string {
	return TruncateMD5(strings.Join(names, "."), 63)
}

func GenerateDNS1123SubDomainName(names ...string) string {
	return TruncateMD5(strings.Join(names, "."), validation.DNS1123SubdomainMaxLength)
}

// TruncateMD5 is called when the string length > maxLen
func TruncateMD5(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	sumHex := md5.Sum([]byte(s))
	sum := hex.EncodeToString(sumHex[:])
	suffix := "-" + sum
	suffixLen := len(suffix)

	front := maxLen - suffixLen

	return s[:front] + suffix
}
