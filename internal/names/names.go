package names

import (
	"crypto/md5"
	"encoding/hex"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

func GenerateDNS1123SubDomainName(names ...string) string {
	generatedName := strings.Join(names, ".")
	if len(generatedName) > validation.DNS1123SubdomainMaxLength {
		generatedName = TruncateMD5(generatedName, validation.DNS1123SubdomainMaxLength)
	}

	return generatedName
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
