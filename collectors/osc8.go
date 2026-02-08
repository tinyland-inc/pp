package collectors

import "fmt"

// Link creates an OSC 8 hyperlink escape sequence.
// Terminal emulators that support OSC 8 (Ghostty, iTerm2, WezTerm, etc.)
// will render the text as a clickable hyperlink that opens the URL.
// Unsupported terminals display the text without the link.
//
// Example:
//
//	Link("https://console.civo.com", "Civo") => clickable "Civo" text
func Link(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}
