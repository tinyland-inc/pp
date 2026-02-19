package shell

import "fmt"

// shGenerateKsh produces the Ksh93 shell integration script.
func shGenerateKsh(opts Options) string {
	s := fmt.Sprintf(`# prompt-pulse shell integration for Ksh93
# eval "$(prompt-pulse shell ksh)" in your ~/.kshrc

`)
	s += shKshBanner(opts)
	s += shKshKeybinding(opts)
	s += shKshCompletions(opts)
	s += shKshDaemonFunctions(opts)
	s += shKshDaemonAutoStart(opts)
	return s
}

// shKshBanner generates the banner display block for Ksh93 using PS1 command
// substitution.
func shKshBanner(opts Options) string {
	if !opts.ShowBanner {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Display banner on shell startup
if [ "${PROMPT_PULSE_BANNER:-1}" != "0" ]; then
    %s -banner 2>/dev/null
fi

# Inline banner via PS1 command substitution
PS1='$(prompt-pulse -banner --inline 2>/dev/null)'"${PS1}"

`, bin)
}

// shKshKeybinding generates the keybinding block for Ksh93 using the KEYBD
// trap.
func shKshKeybinding(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Launch TUI via KEYBD trap (%s)
__prompt_pulse_tui() {
    %s -tui </dev/tty
}
trap '__prompt_pulse_keybd_handler' KEYBD
__prompt_pulse_keybd_handler() {
    # Ctrl+P is ASCII 0x10 (^P)
    case "${.sh.edchar}" in
        $'\x10')
            __prompt_pulse_tui
            ;;
    esac
}

`, opts.Keybinding, bin)
}

// shKshCompletions generates a minimal completion setup for Ksh93.
func shKshCompletions(opts Options) string {
	if !opts.EnableCompletions {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Tab completions (ksh93 set -A pattern)
__prompt_pulse_completions() {
    set -A COMPREPLY -- banner tui daemon shell version
}
complete -C %s prompt-pulse

`, bin)
}

// shKshDaemonFunctions generates the pp-start/pp-stop/pp-status functions
// for Ksh93.
func shKshDaemonFunctions(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Daemon management functions
pp-start() {
    %[1]s -daemon &
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

// shKshDaemonAutoStart generates the auto-start check for Ksh93.
func shKshDaemonAutoStart(opts Options) string {
	if !opts.DaemonAutoStart {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Auto-start daemon if not running
if ! %s -health >/dev/null 2>&1; then
    %s -daemon >/dev/null 2>&1 &
fi

`, bin, bin)
}

// shKshPreexec generates the DEBUG trap for preexec timing in Ksh93.
func shKshPreexec(opts Options) string {
	return `# Preexec timing via DEBUG trap
trap '__prompt_pulse_preexec_ts=$SECONDS' DEBUG

`
}
