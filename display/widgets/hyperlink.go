package widgets

import "fmt"

// RenderHyperlink returns text wrapped in an OSC 8 hyperlink escape sequence.
// Terminals that support OSC 8 (Ghostty, Alacritty, iTerm2, Kitty, WezTerm)
// will render the text as a clickable link. Terminals without support display
// the text normally with no visible artifacts.
//
// Example: RenderHyperlink("https://example.com", "click here")
// renders "click here" as a clickable link to https://example.com.
func RenderHyperlink(url, text string) string {
	if url == "" {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
