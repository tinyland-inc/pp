package shell

import (
	"os"
	"strings"
	"testing"
)

// --- Generate() tests for each shell type ---

func TestGenerate_BashNonEmpty(t *testing.T) {
	out := Generate(Bash, Options{})
	if out == "" {
		t.Fatal("Generate(Bash) returned empty string")
	}
	if !strings.Contains(out, "Bash") {
		t.Error("Bash output should mention Bash in header")
	}
}

func TestGenerate_ZshNonEmpty(t *testing.T) {
	out := Generate(Zsh, Options{})
	if out == "" {
		t.Fatal("Generate(Zsh) returned empty string")
	}
	if !strings.Contains(out, "Zsh") {
		t.Error("Zsh output should mention Zsh in header")
	}
}

func TestGenerate_FishNonEmpty(t *testing.T) {
	out := Generate(Fish, Options{})
	if out == "" {
		t.Fatal("Generate(Fish) returned empty string")
	}
	if !strings.Contains(out, "Fish") {
		t.Error("Fish output should mention Fish in header")
	}
}

func TestGenerate_KshNonEmpty(t *testing.T) {
	out := Generate(Ksh, Options{})
	if out == "" {
		t.Fatal("Generate(Ksh) returned empty string")
	}
	if !strings.Contains(out, "Ksh") {
		t.Error("Ksh output should mention Ksh in header")
	}
}

func TestGenerate_UnknownShell(t *testing.T) {
	out := Generate(ShellType("csh"), Options{})
	if !strings.Contains(out, "not supported") {
		t.Errorf("unknown shell should produce not-supported comment, got: %s", out)
	}
}

// --- Bash-specific content tests ---

func TestBash_ContainsPromptCommand(t *testing.T) {
	out := Generate(Bash, Options{ShowBanner: true})
	if !strings.Contains(out, "PROMPT_COMMAND") {
		t.Error("Bash with ShowBanner should contain PROMPT_COMMAND")
	}
}

func TestBash_ContainsBindX(t *testing.T) {
	out := Generate(Bash, Options{})
	if !strings.Contains(out, "bind -x") {
		t.Error("Bash output should contain bind -x keybinding")
	}
}

func TestBash_ContainsComplete(t *testing.T) {
	out := Generate(Bash, Options{EnableCompletions: true})
	if !strings.Contains(out, "complete -C") {
		t.Error("Bash with EnableCompletions should contain complete -C")
	}
}

// --- Zsh-specific content tests ---

func TestZsh_ContainsAddZshHook(t *testing.T) {
	out := Generate(Zsh, Options{ShowBanner: true})
	if !strings.Contains(out, "add-zsh-hook") {
		t.Error("Zsh with ShowBanner should contain add-zsh-hook")
	}
}

func TestZsh_ContainsZleAndBindkey(t *testing.T) {
	out := Generate(Zsh, Options{})
	if !strings.Contains(out, "zle -N") {
		t.Error("Zsh output should contain zle -N widget registration")
	}
	if !strings.Contains(out, "bindkey") {
		t.Error("Zsh output should contain bindkey")
	}
}

func TestZsh_ContainsCompdef(t *testing.T) {
	out := Generate(Zsh, Options{EnableCompletions: true})
	if !strings.Contains(out, "compdef") {
		t.Error("Zsh with EnableCompletions should contain compdef")
	}
}

func TestZsh_TUIWidgetUsesDevTTY(t *testing.T) {
	out := Generate(Zsh, Options{})
	if !strings.Contains(out, "</dev/tty") {
		t.Error("Zsh TUI widget should read from /dev/tty")
	}
	if !strings.Contains(out, ">/dev/tty") {
		t.Error("Zsh TUI widget should write to /dev/tty")
	}
}

// --- Fish-specific content tests ---

func TestFish_ContainsBind(t *testing.T) {
	out := Generate(Fish, Options{})
	if !strings.Contains(out, "bind") {
		t.Error("Fish output should contain bind command")
	}
}

func TestFish_ContainsFunction(t *testing.T) {
	out := Generate(Fish, Options{})
	if !strings.Contains(out, "function") {
		t.Error("Fish output should contain function keyword")
	}
}

func TestFish_ContainsComplete(t *testing.T) {
	out := Generate(Fish, Options{EnableCompletions: true})
	if !strings.Contains(out, "complete -c prompt-pulse") {
		t.Error("Fish with EnableCompletions should contain complete -c prompt-pulse")
	}
}

func TestFish_BindsAllModes(t *testing.T) {
	out := Generate(Fish, Options{})
	if !strings.Contains(out, "-M insert") {
		t.Error("Fish should bind in insert mode")
	}
	if !strings.Contains(out, "-M visual") {
		t.Error("Fish should bind in visual mode")
	}
}

func TestFish_BannerUsesOnEvent(t *testing.T) {
	out := Generate(Fish, Options{ShowBanner: true})
	if !strings.Contains(out, "--on-event fish_prompt") {
		t.Error("Fish banner should use --on-event fish_prompt")
	}
}

// --- Ksh-specific content tests ---

func TestKsh_ContainsPS1(t *testing.T) {
	out := Generate(Ksh, Options{ShowBanner: true})
	if !strings.Contains(out, "PS1") {
		t.Error("Ksh with ShowBanner should contain PS1")
	}
}

func TestKsh_ContainsTrapKEYBD(t *testing.T) {
	out := Generate(Ksh, Options{})
	if !strings.Contains(out, "trap") {
		t.Error("Ksh output should contain trap for keybinding")
	}
	if !strings.Contains(out, "KEYBD") {
		t.Error("Ksh output should contain KEYBD trap")
	}
}

func TestKsh_BannerInlineSubstitution(t *testing.T) {
	out := Generate(Ksh, Options{ShowBanner: true})
	if !strings.Contains(out, "prompt-pulse -banner --inline 2>/dev/null") {
		t.Error("Ksh banner should use inline command substitution in PS1")
	}
}

// --- Detect() tests ---

func TestDetect_BashShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/bin/bash")
	if got := Detect(); got != Bash {
		t.Errorf("Detect() with SHELL=/bin/bash = %q, want bash", got)
	}
}

func TestDetect_ZshShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/usr/bin/zsh")
	if got := Detect(); got != Zsh {
		t.Errorf("Detect() with SHELL=/usr/bin/zsh = %q, want zsh", got)
	}
}

func TestDetect_FishShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/usr/local/bin/fish")
	if got := Detect(); got != Fish {
		t.Errorf("Detect() with SHELL=/usr/local/bin/fish = %q, want fish", got)
	}
}

func TestDetect_KshShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/bin/ksh93")
	if got := Detect(); got != Ksh {
		t.Errorf("Detect() with SHELL=/bin/ksh93 = %q, want ksh", got)
	}
}

func TestDetect_LoginShellDash(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	// Login shells are sometimes reported with a leading dash.
	os.Setenv("SHELL", "/bin/-zsh")
	if got := Detect(); got != Zsh {
		t.Errorf("Detect() with SHELL=/bin/-zsh = %q, want zsh", got)
	}
}

func TestDetect_FallbackBash(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "")
	// With empty $SHELL and no /proc, Detect will fall back to Bash.
	got := Detect()
	// On macOS, the parent process detection might succeed.
	// We just verify it returns a valid shell type.
	if got != Bash && got != Zsh && got != Fish && got != Ksh {
		t.Errorf("Detect() with empty SHELL = %q, want a valid ShellType", got)
	}
}

func TestDetect_UnknownShellFallback(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/usr/bin/tcsh")
	// tcsh is not recognized, so shDetectFromEnv returns "", but
	// the parent process detection might succeed. Just verify we get
	// a valid type.
	got := Detect()
	if got != Bash && got != Zsh && got != Fish && got != Ksh {
		t.Errorf("Detect() with unknown shell = %q, want a valid ShellType", got)
	}
}

// --- Options default tests ---

func TestDefaultOptions_BashKeybinding(t *testing.T) {
	opts := shDefaultOptions(Bash, Options{})
	if opts.Keybinding != `\C-p` {
		t.Errorf("Bash default keybinding = %q, want \\C-p", opts.Keybinding)
	}
}

func TestDefaultOptions_FishKeybinding(t *testing.T) {
	opts := shDefaultOptions(Fish, Options{})
	if opts.Keybinding != "ctrl-p" {
		t.Errorf("Fish default keybinding = %q, want ctrl-p", opts.Keybinding)
	}
}

func TestDefaultOptions_BinaryPath(t *testing.T) {
	opts := shDefaultOptions(Bash, Options{})
	if opts.BinaryPath != "prompt-pulse" {
		t.Errorf("default BinaryPath = %q, want prompt-pulse", opts.BinaryPath)
	}
}

// --- ShowBanner=false omits banner code ---

func TestShowBannerFalse_Bash(t *testing.T) {
	out := Generate(Bash, Options{ShowBanner: false})
	if strings.Contains(out, "PROMPT_PULSE_BANNER") {
		t.Error("Bash with ShowBanner=false should not contain banner code")
	}
}

func TestShowBannerFalse_Zsh(t *testing.T) {
	out := Generate(Zsh, Options{ShowBanner: false})
	if strings.Contains(out, "add-zsh-hook") {
		t.Error("Zsh with ShowBanner=false should not contain add-zsh-hook")
	}
}

func TestShowBannerFalse_Fish(t *testing.T) {
	out := Generate(Fish, Options{ShowBanner: false})
	if strings.Contains(out, "__prompt_pulse_banner") {
		t.Error("Fish with ShowBanner=false should not contain banner function")
	}
}

func TestShowBannerFalse_Ksh(t *testing.T) {
	out := Generate(Ksh, Options{ShowBanner: false})
	if strings.Contains(out, "PS1") {
		t.Error("Ksh with ShowBanner=false should not contain PS1 modification")
	}
}

// --- DaemonAutoStart=true includes daemon check ---

func TestDaemonAutoStart_Bash(t *testing.T) {
	out := Generate(Bash, Options{DaemonAutoStart: true})
	if !strings.Contains(out, "-daemon") {
		t.Error("Bash with DaemonAutoStart should contain -daemon flag")
	}
	if !strings.Contains(out, "-health") {
		t.Error("Bash with DaemonAutoStart should check health status")
	}
}

func TestDaemonAutoStart_Zsh(t *testing.T) {
	out := Generate(Zsh, Options{DaemonAutoStart: true})
	if !strings.Contains(out, "-daemon") {
		t.Error("Zsh with DaemonAutoStart should contain -daemon flag")
	}
}

func TestDaemonAutoStart_Fish(t *testing.T) {
	out := Generate(Fish, Options{DaemonAutoStart: true})
	if !strings.Contains(out, "-daemon") {
		t.Error("Fish with DaemonAutoStart should contain -daemon flag")
	}
}

func TestDaemonAutoStart_Ksh(t *testing.T) {
	out := Generate(Ksh, Options{DaemonAutoStart: true})
	if !strings.Contains(out, "-daemon") {
		t.Error("Ksh with DaemonAutoStart should contain -daemon flag")
	}
}

// --- DaemonAutoStart=false omits autostart ---

func TestDaemonAutoStartFalse_Bash(t *testing.T) {
	out := Generate(Bash, Options{DaemonAutoStart: false})
	if strings.Contains(out, "Auto-start daemon") {
		t.Error("Bash with DaemonAutoStart=false should not contain auto-start block")
	}
}

// --- EnableCompletions=false omits completion code ---

func TestCompletionsFalse_Bash(t *testing.T) {
	out := Generate(Bash, Options{EnableCompletions: false})
	if strings.Contains(out, "complete -C") {
		t.Error("Bash with EnableCompletions=false should not contain complete")
	}
}

func TestCompletionsFalse_Zsh(t *testing.T) {
	out := Generate(Zsh, Options{EnableCompletions: false})
	if strings.Contains(out, "compdef") {
		t.Error("Zsh with EnableCompletions=false should not contain compdef")
	}
}

func TestCompletionsFalse_Fish(t *testing.T) {
	out := Generate(Fish, Options{EnableCompletions: false})
	if strings.Contains(out, "complete -c prompt-pulse") {
		t.Error("Fish with EnableCompletions=false should not contain complete")
	}
}

func TestCompletionsFalse_Ksh(t *testing.T) {
	out := Generate(Ksh, Options{EnableCompletions: false})
	if strings.Contains(out, "complete") {
		t.Error("Ksh with EnableCompletions=false should not contain complete")
	}
}

// --- All scripts contain pp-start, pp-stop, pp-status ---

func TestDaemonFunctions_AllShells(t *testing.T) {
	shells := []ShellType{Bash, Zsh, Fish, Ksh}
	fns := []string{"pp-start", "pp-stop", "pp-status"}

	for _, sh := range shells {
		out := Generate(sh, Options{})
		for _, fn := range fns {
			if !strings.Contains(out, fn) {
				t.Errorf("%s output missing function %q", sh, fn)
			}
		}
	}
}

// --- Custom keybinding overrides ---

func TestCustomKeybinding_Bash(t *testing.T) {
	out := Generate(Bash, Options{Keybinding: `\C-g`})
	if !strings.Contains(out, `\C-g`) {
		t.Error("Bash should use custom keybinding \\C-g")
	}
}

func TestCustomKeybinding_Fish(t *testing.T) {
	out := Generate(Fish, Options{Keybinding: "ctrl-g"})
	if !strings.Contains(out, `\cg`) {
		t.Error("Fish should convert ctrl-g to \\cg in bind command")
	}
}

// --- BinaryPath is properly quoted ---

func TestBinaryPathWithSpaces_Bash(t *testing.T) {
	out := Generate(Bash, Options{BinaryPath: "/path/to/my prompt pulse"})
	if !strings.Contains(out, "'/path/to/my prompt pulse'") {
		t.Error("Bash should single-quote binary path containing spaces")
	}
}

func TestBinaryPathWithSpaces_Zsh(t *testing.T) {
	out := Generate(Zsh, Options{BinaryPath: "/my path/prompt-pulse"})
	if !strings.Contains(out, "'/my path/prompt-pulse'") {
		t.Error("Zsh should single-quote binary path containing spaces")
	}
}

func TestBinaryPathWithSpaces_Fish(t *testing.T) {
	out := Generate(Fish, Options{BinaryPath: "/path with spaces/pp"})
	if !strings.Contains(out, "'/path with spaces/pp'") {
		t.Error("Fish should single-quote binary path containing spaces")
	}
}

func TestBinaryPathWithSpaces_Ksh(t *testing.T) {
	out := Generate(Ksh, Options{BinaryPath: "/opt/my tools/pp"})
	if !strings.Contains(out, "'/opt/my tools/pp'") {
		t.Error("Ksh should single-quote binary path containing spaces")
	}
}

// --- BinaryPath with single quotes is properly escaped ---

func TestBinaryPathWithSingleQuote(t *testing.T) {
	out := Generate(Bash, Options{BinaryPath: "/it's/here"})
	// The shQuote function should produce: '/it'\''s/here'
	if !strings.Contains(out, `'/it'\''s/here'`) {
		t.Errorf("single quotes in binary path should be escaped, got: %s", out)
	}
}

// --- Structural validation ---

func TestAllShells_NoEmptyOutput(t *testing.T) {
	shells := []ShellType{Bash, Zsh, Fish, Ksh}
	for _, sh := range shells {
		out := Generate(sh, Options{
			ShowBanner:        true,
			DaemonAutoStart:   true,
			EnableCompletions: true,
		})
		if len(out) < 100 {
			t.Errorf("%s output is suspiciously short (%d bytes)", sh, len(out))
		}
	}
}

func TestAllShells_ContainHeader(t *testing.T) {
	shells := []ShellType{Bash, Zsh, Fish, Ksh}
	for _, sh := range shells {
		out := Generate(sh, Options{})
		if !strings.HasPrefix(out, "# prompt-pulse shell integration") {
			t.Errorf("%s output should start with header comment", sh)
		}
	}
}

// --- shParseShellName tests ---

func TestParseShellName(t *testing.T) {
	tests := []struct {
		input string
		want  ShellType
	}{
		{"bash", Bash},
		{"zsh", Zsh},
		{"fish", Fish},
		{"ksh", Ksh},
		{"ksh93", Ksh},
		{"mksh", Ksh},
		{"-zsh", Zsh},
		{"-bash", Bash},
		{"tcsh", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := shParseShellName(tt.input)
		if got != tt.want {
			t.Errorf("shParseShellName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- shQuote tests ---

func TestShQuote_Simple(t *testing.T) {
	got := shQuote("hello")
	if got != "'hello'" {
		t.Errorf("shQuote(hello) = %q, want 'hello'", got)
	}
}

func TestShQuote_WithSpaces(t *testing.T) {
	got := shQuote("/path/to/my binary")
	if got != "'/path/to/my binary'" {
		t.Errorf("shQuote with spaces = %q", got)
	}
}

func TestShQuote_WithSingleQuotes(t *testing.T) {
	got := shQuote("it's")
	want := `'it'\''s'`
	if got != want {
		t.Errorf("shQuote(it's) = %q, want %q", got, want)
	}
}

// --- shFishKeySequence tests ---

func TestFishKeySequence(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ctrl-p", `\cp`},
		{`\C-p`, `\cp`},
		{"ctrl-g", `\cg`},
		{`\C-g`, `\cg`},
		{`\co`, `\co`},
	}
	for _, tt := range tests {
		got := shFishKeySequence(tt.input)
		if got != tt.want {
			t.Errorf("shFishKeySequence(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
