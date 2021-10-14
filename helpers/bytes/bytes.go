// Package bytes is a helper to convert bytes to a human readable string
package bytes

import "fmt"

// ByteCountIEC converts a size in bytes to a human-readable string in IEC (binary) format.
// Copied from:
// https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
// https://creativecommons.org/licenses/by/3.0/
// IEC format: https://www.iec.ch/prefixes-binary-multiples
func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
