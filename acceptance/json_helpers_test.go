package acceptance_test

import "strings"

// extractJSONPayload attempts to strip any non-JSON noise (prompts, logs, emojis, etc.)
// from CLI output and return just the first complete JSON payload so that json.Unmarshal can succeed.
func extractJSONPayload(out string) string {
	start := strings.IndexAny(out, "{[")
	if start == -1 {
		return out
	}

	open := out[start]
	var close byte
	if open == '{' {
		close = '}'
	} else {
		close = ']'
	}

	depth := 0
	inString := false
	escape := false

	for i := start; i < len(out); i++ {
		c := out[i]

		if escape {
			escape = false
			continue
		}

		if c == '\\' && inString {
			escape = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return out[start : i+1]
			}
		}
	}

	// Fallback: return from first JSON starter to end if we couldn't balance,
	// so tests at least see something close to what was printed.
	return out[start:]
}
