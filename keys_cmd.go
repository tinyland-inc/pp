package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
)

// runKeysCommand prints all keybindings to stdout.
func runKeysCommand(mode string, format string) {
	reg := tui.DefaultRegistry()

	switch format {
	case "json":
		entries := reg.FormatJSON()
		if mode != "" {
			entries = filterJSONByMode(entries, mode)
		}
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))

	default: // "table"
		if mode != "" {
			filtered := reg.ByMode(tui.KeyMode(mode))
			if len(filtered) == 0 {
				fmt.Fprintf(os.Stderr, "no bindings found for mode %q\n", mode)
				os.Exit(1)
			}
			// Build a temporary registry with just this mode.
			tmp := &tui.KeyRegistry{Entries: filtered}
			fmt.Print(tmp.FormatTable())
		} else {
			fmt.Print(reg.FormatTable())
		}
	}
}

// filterJSONByMode filters FormatJSON entries by mode name.
func filterJSONByMode(entries []map[string]string, mode string) []map[string]string {
	var result []map[string]string
	for _, e := range entries {
		if e["mode"] == mode {
			result = append(result, e)
		}
	}
	return result
}
