package helpers

import "strings"

func Retryable(msg string) bool {
	return strings.Contains(msg, " x509: ") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "Gateway") ||
		strings.Contains(msg, "Service Unavailable") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "no endpoints available") ||
		strings.Contains(msg, "x509: certificate signed by unknown authority") ||
		(strings.Contains(msg, "api/v1/namespaces") && strings.Contains(msg, "i/o timeout"))
}

func RetryableCode(code int) bool {
	return code >= 400
}
