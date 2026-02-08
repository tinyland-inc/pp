package format

// Truncate truncates a string to maxLen characters.
// Returns the full string if it's shorter than maxLen.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// TruncateRunes truncates a string to maxLen runes (Unicode-aware).
// Returns the full string if it's shorter than maxLen runes.
func TruncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// TruncateWithEllipsis truncates a string to maxWidth characters, appending "..."
// if the string exceeds the limit. If maxWidth is less than 4, the string
// is hard-truncated without an ellipsis suffix.
func TruncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}

	if maxWidth < 4 {
		return string(runes[:maxWidth])
	}

	return string(runes[:maxWidth-3]) + "..."
}

// UniqueStrings returns a deduplicated slice of strings.
// The order of first occurrence is preserved.
func UniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
