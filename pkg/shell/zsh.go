package shell

import "fmt"

// shGenerateZsh produces the Zsh shell integration script.
func shGenerateZsh(opts Options) string {
	s := fmt.Sprintf(`# prompt-pulse shell integration for Zsh
# eval "$(prompt-pulse shell zsh)" in your ~/.zshrc

`)
	s += shZshBanner(opts)
	s += shZshKeybinding(opts)
	s += shZshCompletions(opts)
	s += shZshDaemonFunctions(opts)
	s += shZshDaemonAutoStart(opts)
	return s
}

// shZshBanner generates the banner display block for Zsh.
func shZshBanner(opts Options) string {
	if !opts.ShowBanner {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Display banner on shell startup
if [[ "${PROMPT_PULSE_BANNER:-1}" != "0" ]]; then
    %s -banner 2>/dev/null
fi

# Refresh hook via add-zsh-hook
autoload -Uz add-zsh-hook
__prompt_pulse_precmd() {
    true
}
add-zsh-hook precmd __prompt_pulse_precmd

`, bin)
}

// shZshKeybinding generates the keybinding block for Zsh using a ZLE widget
// with proper /dev/tty redirection.
func shZshKeybinding(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# ZLE widget for TUI launch with /dev/tty redirection
__prompt_pulse_tui_widget() {
    BUFFER=""
    zle reset-prompt
    %s -tui </dev/tty >/dev/tty 2>/dev/tty
    zle reset-prompt
}
zle -N prompt-pulse-tui __prompt_pulse_tui_widget
bindkey '^P' prompt-pulse-tui

`, bin)
}

// shZshCompletions generates the completion block for Zsh.
func shZshCompletions(opts Options) string {
	if !opts.EnableCompletions {
		return ""
	}
	return `# Tab completions
_prompt_pulse_complete() {
    local -a commands
    commands=(
        'banner:Display system status banner'
        'tui:Launch interactive TUI dashboard'
        'daemon:Manage background daemon'
        'shell:Generate shell integration'
        'version:Show version information'
    )
    _describe 'prompt-pulse' commands
}
compdef _prompt_pulse_complete prompt-pulse

`
}

// shZshDaemonFunctions generates the pp-start/pp-stop/pp-status functions
// for Zsh.
func shZshDaemonFunctions(opts Options) string {
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Daemon management functions
pp-start() {
    %[1]s -daemon &!
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

// shZshDaemonAutoStart generates the auto-start check for Zsh.
func shZshDaemonAutoStart(opts Options) string {
	if !opts.DaemonAutoStart {
		return ""
	}
	bin := shQuote(opts.BinaryPath)
	return fmt.Sprintf(`# Auto-start daemon if not running
if ! %s -health >/dev/null 2>&1; then
    %s -daemon >/dev/null 2>&1 &!
fi

`, bin, bin)
}
