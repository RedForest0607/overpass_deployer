package ssh

import "strings"

// ShellQuote escapes a string for safe single-argument shell usage.
func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
