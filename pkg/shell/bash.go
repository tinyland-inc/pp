package shell

import "fmt"

// shGenerateBash produces the Bash shell integration script.
func shGenerateBash(opts Options) string {
	s := fmt.Sprintf(`# prompt-pulse shell integration for Bash
# eval "$(prompt-pulse shell bash)" in your ~/.bashrc

`)
	s += shBashBanner(opts)
	s += shBashKeybinding(opts)
	s += shBashCompletions(opts)
	s += shBashDaemonFunctions(opts)
	s += shBashDaemonAutoStart(opts)
	return s
}

// shBashBanner generates the banner display block for Bash.
func shBashBanner(opts Options) string {
	if !opts.ShowBanner {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Display banner on shell startup
if [ "${PROMPT_PULSE_BANNER:-1}" != "0" ]; then
    %s -banner 2>/dev/null
fi

# Append banner refresh to PROMPT_COMMAND (without clobbering existing)
__prompt_pulse_precmd() {
    true
}
if [[ "$PROMPT_COMMAND" != *"__prompt_pulse_precmd"* ]]; then
    PROMPT_COMMAND="__prompt_pulse_precmd;${PROMPT_COMMAND:-}"
fi

`, bin)
}

// shBashKeybinding generates the keybinding block for Bash.
func shBashKeybinding(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Launch TUI with keybinding (%s)
__prompt_pulse_tui() {
    %s -tui </dev/tty
}
bind -x '"%s": __prompt_pulse_tui'

`, opts.Keybinding, bin, opts.Keybinding)
}

// shBashCompletions generates the completion block for Bash.
func shBashCompletions(opts Options) string {
	if !opts.EnableCompletions {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Tab completions
complete -C %s prompt-pulse

`, bin)
}

// shBashDaemonFunctions generates the pp-start/pp-stop/pp-status functions
// for Bash.
func shBashDaemonFunctions(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Daemon management functions
pp-start() {
    %[1]s -daemon &
    disown
    echo "prompt-pulse daemon started (PID $!)"
}

pp-stop() {
    pkill -f '%[1]s -daemon' 2>/dev/null && echo "prompt-pulse daemon stopped" || echo "daemon not running"
}

pp-status() {
    %[1]s -health
}

pp-banner() {
    %[1]s -banner
}

`, bin)
}

// shBashDaemonAutoStart generates the auto-start check for Bash.
func shBashDaemonAutoStart(opts Options) string {
	if !opts.DaemonAutoStart {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Auto-start daemon if not running
if ! %s -health >/dev/null 2>&1; then
    %s -daemon >/dev/null 2>&1 &
    disown
fi

`, bin, bin)
}
