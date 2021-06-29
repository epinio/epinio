package helpers

import (
	"strings"

	"github.com/spf13/viper"
)

func MagicDomain() string {
	return viper.GetString("magic-domain")
}

func IsMagicDomain(domain string) bool {
	return strings.Contains(domain, MagicDomain())
}
