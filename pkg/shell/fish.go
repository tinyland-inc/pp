package shell

import "fmt"

// shGenerateFish produces the Fish shell integration script.
func shGenerateFish(opts Options) string {
	s := fmt.Sprintf(`# prompt-pulse shell integration for Fish
# prompt-pulse shell fish | source  (in your config.fish)

`)
	s += shFishBanner(opts)
	s += shFishKeybinding(opts)
	s += shFishWaifuKeybinding(opts)
	s += shFishCompletions(opts)
	s += shFishDaemonFunctions(opts)
	s += shFishDaemonAutoStart(opts)
	return s
}

// shFishBanner generates the banner display block for Fish using the
// fish_prompt event.
func shFishBanner(opts Options) string {
	if !opts.ShowBanner {
		return ""
	}
	bin := shFishQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Display banner on shell startup via fish_prompt event
function __prompt_pulse_banner --on-event fish_prompt
    if test "$PROMPT_PULSE_BANNER" != "0"
        %s -banner 2>/dev/null
    end
    # Only show banner once per session; remove handler after first call
    functions -e __prompt_pulse_banner
end

`, bin)
}

// shFishKeybinding generates the keybinding block for Fish, binding in all
// three modes (default, insert, visual).
func shFishKeybinding(opts Options) string {
	// Convert the keybinding to fish format (e.g. "ctrl-p" -> \cp).
	fishKey := shFishKeySequence(opts.Keybinding)
	return fmt.Sprintf(`# Launch TUI with keybinding (%s)
function __prompt_pulse_tui
    commandline -f repaint
    if command -q prompt-pulse-tui
        prompt-pulse-tui </dev/tty >/dev/tty 2>/dev/tty
    end
    commandline -f repaint
end
bind %s __prompt_pulse_tui
bind -M insert %s __prompt_pulse_tui
bind -M visual %s __prompt_pulse_tui

`, opts.Keybinding, fishKey, fishKey, fishKey)
}

// shFishWaifuKeybinding generates a keybinding that launches the TUI with
// the waifu widget expanded (ctrl-w by default).
func shFishWaifuKeybinding(opts Options) string {
	if opts.WaifuKeybinding == "" {
		return ""
	}
	fishKey := shFishKeySequence(opts.WaifuKeybinding)
	return fmt.Sprintf(`# Launch TUI with waifu expanded (%s)
function __prompt_pulse_waifu
    commandline -f repaint
    if command -q prompt-pulse-tui
        prompt-pulse-tui --expand waifu </dev/tty >/dev/tty 2>/dev/tty
    end
    commandline -f repaint
end
bind %s __prompt_pulse_waifu
bind -M insert %s __prompt_pulse_waifu
bind -M visual %s __prompt_pulse_waifu

`, opts.WaifuKeybinding, fishKey, fishKey, fishKey)
}

// shFishCompletions generates the completion block for Fish.
func shFishCompletions(opts Options) string {
	if !opts.EnableCompletions {
		return ""
	}
	return `# Tab completions (flag-based CLI)
complete -c prompt-pulse -l banner -d "Display system status banner"
complete -c prompt-pulse -l tui -d "Launch interactive TUI dashboard"
complete -c prompt-pulse -l daemon -d "Run background daemon"
complete -c prompt-pulse -l shell -r -d "Generate shell integration (bash|zsh|fish|ksh)"
complete -c prompt-pulse -l version -d "Show version information"
complete -c prompt-pulse -l health -d "Check daemon health status"

`
}

// shFishDaemonFunctions generates the pp-start/pp-stop/pp-status functions
// for Fish.
func shFishDaemonFunctions(opts Options) string {
	bin := shFishQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Daemon management functions
function pp-start -d "Start prompt-pulse daemon"
    %[1]s -daemon &
    disown
    echo "prompt-pulse daemon started"
end

function pp-stop -d "Stop prompt-pulse daemon"
    pkill -f '%[1]s -daemon' 2>/dev/null; and echo "prompt-pulse daemon stopped"; or echo "daemon not running"
end

function pp-status -d "Show prompt-pulse daemon status"
    %[1]s -health
end

function pp-banner -d "Display prompt-pulse banner"
    %[1]s -banner
end

`, bin)
}

// shFishDaemonAutoStart generates the auto-start check for Fish.
func shFishDaemonAutoStart(opts Options) string {
	if !opts.DaemonAutoStart {
		return ""
	}
	bin := shFishQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Auto-start daemon if not running
if not %s -health >/dev/null 2>&1
    %s -daemon >/dev/null 2>&1 &
    disown
end

`, bin, bin)
}

// shFishQuote returns the binary path suitable for Fish. Fish uses the same
// quoting rules as POSIX for single quotes.
func shFishQuote(s string) string {
	return shQuote(s)
}

// shFishKeySequence converts a keybinding spec to Fish's bind format.
// "ctrl-p" -> "\cp", "\C-p" -> "\cp".
func shFishKeySequence(kb string) string {
	switch kb {
	case "ctrl-p", `\C-p`:
		return `\cp`
	case "ctrl-w", `\C-w`:
		return `\cw`
	case "ctrl-g", `\C-g`:
		return `\cg`
	case "ctrl-o", `\C-o`:
		return `\co`
	default:
		// If it already looks like a fish escape (\cx), pass through.
		if len(kb) == 3 && kb[0] == '\\' && kb[1] == 'c' {
			return kb
		}
		return `\cp`
	}
}
