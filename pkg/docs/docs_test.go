package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Generator framework tests
// ---------------------------------------------------------------------------

func TestNewCreatesGenerator(t *testing.T) {
	g := New("/tmp/docs")
	if g.OutputDir != "/tmp/docs" {
		t.Errorf("OutputDir = %q, want /tmp/docs", g.OutputDir)
	}
	if g.Format != "markdown" {
		t.Errorf("Format = %q, want markdown", g.Format)
	}
	if len(g.Sections) != 0 {
		t.Errorf("Sections len = %d, want 0", len(g.Sections))
	}
}

func TestAddSection(t *testing.T) {
	g := New("/tmp/docs")
	g.Add("Architecture", "arch", "Content here", 1)
	g.Add("Config", "config", "Config content", 2)

	if len(g.Sections) != 2 {
		t.Fatalf("Sections len = %d, want 2", len(g.Sections))
	}
	if g.Sections[0].Title != "Architecture" {
		t.Errorf("Section 0 title = %q, want Architecture", g.Sections[0].Title)
	}
	if g.Sections[1].Slug != "config" {
		t.Errorf("Section 1 slug = %q, want config", g.Sections[1].Slug)
	}
}

func TestGenerateCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)
	g.Add("First", "first", "First content", 1)
	g.Add("Second", "second", "Second content", 2)

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	for _, slug := range []string{"first", "second"} {
		path := filepath.Join(dir, slug+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("ReadFile(%s) error: %v", path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", path)
		}
	}
}

func TestGenerateCreatesOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	g := New(dir)
	g.Add("Test", "test", "Content", 1)

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%s) error: %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", dir)
	}
}

func TestGenerateSingleCombinesAllSections(t *testing.T) {
	g := New("/tmp/docs")
	g.Add("Architecture", "arch", "Arch content", 1)
	g.Add("Config", "config", "Config content", 2)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}

	if !strings.Contains(doc, "# prompt-pulse Documentation") {
		t.Error("missing main heading")
	}
	if !strings.Contains(doc, "Architecture") {
		t.Error("missing Architecture section")
	}
	if !strings.Contains(doc, "Config") {
		t.Error("missing Config section")
	}
	if !strings.Contains(doc, "Table of Contents") {
		t.Error("missing table of contents")
	}
}

func TestSectionOrdering(t *testing.T) {
	g := New("/tmp/docs")
	g.Add("Third", "third", "3", 3)
	g.Add("First", "first", "1", 1)
	g.Add("Second", "second", "2", 2)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}

	firstIdx := strings.Index(doc, "First")
	secondIdx := strings.Index(doc, "Second")
	thirdIdx := strings.Index(doc, "Third")

	if firstIdx > secondIdx || secondIdx > thirdIdx {
		t.Errorf("sections out of order: first=%d second=%d third=%d", firstIdx, secondIdx, thirdIdx)
	}
}

func TestSubSections(t *testing.T) {
	g := New("/tmp/docs")
	g.AddSection(Section{
		Title:   "Parent",
		Slug:    "parent",
		Content: "Parent content",
		Order:   1,
		SubSections: []Section{
			{Title: "Child A", Slug: "child-a", Content: "A content", Order: 1},
			{Title: "Child B", Slug: "child-b", Content: "B content", Order: 2},
		},
	})

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}

	if !strings.Contains(doc, "### Child A") {
		t.Error("missing Child A subsection")
	}
	if !strings.Contains(doc, "### Child B") {
		t.Error("missing Child B subsection")
	}
}

// ---------------------------------------------------------------------------
// Architecture doc tests
// ---------------------------------------------------------------------------

func TestArchDocAll29Packages(t *testing.T) {
	doc := dcGenerateArchDoc()
	// 24 top-level packages + 5 collector sub-packages = 29 entries
	if len(doc.Packages) != 29 {
		t.Errorf("package count = %d, want 29", len(doc.Packages))
	}

	// Verify some key packages exist
	names := make(map[string]bool)
	for _, p := range doc.Packages {
		names[p.Name] = true
	}

	required := []string{
		"layout", "terminal", "config", "app",
		"image", "components", "theme",
		"data", "cache",
		"shell", "starship", "banner",
		"emacs", "daemon",
		"perf", "termtest", "shelltest", "inttest",
		"platform", "sysinfo",
		"nixpkg", "homebrew", "migrate", "docs",
	}

	for _, name := range required {
		if !names[name] {
			t.Errorf("missing package: %s", name)
		}
	}
}

func TestArchDocCollectorPackages(t *testing.T) {
	doc := dcGenerateArchDoc()
	collectors := make(map[string]bool)
	for _, p := range doc.Packages {
		if strings.HasPrefix(p.Name, "collectors/") {
			collectors[p.Name] = true
		}
	}

	expected := []string{
		"collectors/tailscale",
		"collectors/k8s",
		"collectors/claude",
		"collectors/billing",
		"collectors/sysmetrics",
	}
	for _, name := range expected {
		if !collectors[name] {
			t.Errorf("missing collector: %s", name)
		}
	}
}

func TestArchDocLayerAssignments(t *testing.T) {
	doc := dcGenerateArchDoc()

	if len(doc.Layers) != 8 {
		t.Errorf("layer count = %d, want 8", len(doc.Layers))
	}

	layerNames := make(map[string]bool)
	for _, l := range doc.Layers {
		layerNames[l.Name] = true
	}

	expected := []string{
		"Core", "Rendering", "Data", "Shell",
		"Integration", "Testing", "Platform", "Packaging",
	}
	for _, name := range expected {
		if !layerNames[name] {
			t.Errorf("missing layer: %s", name)
		}
	}
}

func TestArchDocDependencyGraph(t *testing.T) {
	doc := dcGenerateArchDoc()
	if doc.Diagram == "" {
		t.Error("dependency diagram is empty")
	}
	if !strings.Contains(doc.Diagram, "app") {
		t.Error("diagram missing app node")
	}
	if !strings.Contains(doc.Diagram, "daemon") {
		t.Error("diagram missing daemon node")
	}
}

func TestArchDocPackageDescriptions(t *testing.T) {
	doc := dcGenerateArchDoc()
	for _, p := range doc.Packages {
		if p.Description == "" {
			t.Errorf("package %s has empty description", p.Name)
		}
		if p.Path == "" {
			t.Errorf("package %s has empty path", p.Name)
		}
	}
}

func TestArchDocPackageExportedTypes(t *testing.T) {
	doc := dcGenerateArchDoc()
	for _, p := range doc.Packages {
		if len(p.ExportedTypes) == 0 {
			t.Errorf("package %s has no exported types", p.Name)
		}
	}
}

func TestRenderArchMarkdown(t *testing.T) {
	doc := dcGenerateArchDoc()
	md := dcRenderArchMarkdown(doc)

	if !strings.Contains(md, "# Architecture") {
		t.Error("missing main heading")
	}
	if !strings.Contains(md, "## Layers") {
		t.Error("missing Layers section")
	}
	if !strings.Contains(md, "## Dependency Diagram") {
		t.Error("missing Dependency Diagram section")
	}
	if !strings.Contains(md, "## Package Reference") {
		t.Error("missing Package Reference section")
	}
	if !strings.Contains(md, "```") {
		t.Error("missing code block for diagram")
	}
}

// ---------------------------------------------------------------------------
// Config reference tests
// ---------------------------------------------------------------------------

func TestConfigRefAllSections(t *testing.T) {
	ref := dcGenerateConfigRef()

	expected := []string{
		"general",
		"layout",
		"collectors.sysmetrics",
		"collectors.tailscale",
		"collectors.kubernetes",
		"collectors.claude",
		"collectors.billing",
		"image",
		"theme",
		"shell",
		"banner",
	}

	if len(ref.Sections) != len(expected) {
		t.Errorf("section count = %d, want %d", len(ref.Sections), len(expected))
	}

	names := make(map[string]bool)
	for _, s := range ref.Sections {
		names[s.Name] = true
	}

	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing config section: %s", name)
		}
	}
}

func TestConfigRefFieldsDocumented(t *testing.T) {
	ref := dcGenerateConfigRef()
	for _, s := range ref.Sections {
		if len(s.Fields) == 0 {
			t.Errorf("section %s has no fields", s.Name)
		}
		for _, f := range s.Fields {
			if f.Name == "" {
				t.Errorf("section %s has field with empty name", s.Name)
			}
			if f.Type == "" {
				t.Errorf("section %s field %s has empty type", s.Name, f.Name)
			}
			if f.Description == "" {
				t.Errorf("section %s field %s has empty description", s.Name, f.Name)
			}
		}
	}
}

func TestConfigRefTOMLExamples(t *testing.T) {
	ref := dcGenerateConfigRef()
	for _, s := range ref.Sections {
		hasExample := false
		for _, f := range s.Fields {
			if f.Example != "" {
				hasExample = true
				// Verify the example contains an = sign (basic TOML validity)
				if !strings.Contains(f.Example, "=") && !strings.HasPrefix(f.Example, "#") {
					t.Errorf("section %s field %s example missing '=': %s", s.Name, f.Name, f.Example)
				}
			}
		}
		if !hasExample {
			t.Errorf("section %s has no fields with examples", s.Name)
		}
	}
}

func TestConfigRefSectionDescriptions(t *testing.T) {
	ref := dcGenerateConfigRef()
	for _, s := range ref.Sections {
		if s.Description == "" {
			t.Errorf("section %s has empty description", s.Name)
		}
	}
}

func TestRenderConfigMarkdown(t *testing.T) {
	ref := dcGenerateConfigRef()
	md := dcRenderConfigMarkdown(ref)

	if !strings.Contains(md, "# Configuration Reference") {
		t.Error("missing main heading")
	}
	if !strings.Contains(md, "```toml") {
		t.Error("missing TOML code blocks")
	}
	if !strings.Contains(md, "| Key | Type |") {
		t.Error("missing fields table header")
	}
	// Ensure all sections appear
	for _, s := range ref.Sections {
		if !strings.Contains(md, fmt.Sprintf("`[%s]`", s.Name)) {
			t.Errorf("markdown missing section heading for %s", s.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Shell guide tests
// ---------------------------------------------------------------------------

func TestShellGuideAll4Shells(t *testing.T) {
	guide := dcGenerateShellGuide()

	if len(guide.Shells) != 4 {
		t.Fatalf("shell count = %d, want 4", len(guide.Shells))
	}

	names := make(map[string]bool)
	for _, s := range guide.Shells {
		names[s.Name] = true
	}

	for _, name := range []string{"Bash", "Zsh", "Fish", "Ksh"} {
		if !names[name] {
			t.Errorf("missing shell: %s", name)
		}
	}
}

func TestShellGuideSetupCommands(t *testing.T) {
	guide := dcGenerateShellGuide()
	for _, s := range guide.Shells {
		if s.SetupCommand == "" {
			t.Errorf("shell %s has empty setup command", s.Name)
		}
		if !strings.Contains(s.SetupCommand, "prompt-pulse") {
			t.Errorf("shell %s setup command doesn't reference prompt-pulse: %s", s.Name, s.SetupCommand)
		}
	}
}

func TestShellGuideConfigFiles(t *testing.T) {
	guide := dcGenerateShellGuide()
	for _, s := range guide.Shells {
		if s.ConfigFile == "" {
			t.Errorf("shell %s has empty config file", s.Name)
		}
	}
}

func TestShellGuideFeatures(t *testing.T) {
	guide := dcGenerateShellGuide()
	for _, s := range guide.Shells {
		if len(s.Features) == 0 {
			t.Errorf("shell %s has no features listed", s.Name)
		}
	}
}

func TestShellGuideCaveats(t *testing.T) {
	guide := dcGenerateShellGuide()
	for _, s := range guide.Shells {
		if len(s.Caveats) == 0 {
			t.Errorf("shell %s has no caveats listed", s.Name)
		}
	}
}

func TestShellGuideHookTypes(t *testing.T) {
	guide := dcGenerateShellGuide()
	expectedHooks := map[string]string{
		"Bash": "PROMPT_COMMAND",
		"Zsh":  "precmd",
		"Fish": "fish_prompt",
		"Ksh":  "PS1",
	}
	for _, s := range guide.Shells {
		expected, ok := expectedHooks[s.Name]
		if !ok {
			continue
		}
		if !strings.Contains(s.HookType, expected) {
			t.Errorf("shell %s hook type %q doesn't contain %q", s.Name, s.HookType, expected)
		}
	}
}

func TestRenderShellMarkdown(t *testing.T) {
	guide := dcGenerateShellGuide()
	md := dcRenderShellMarkdown(guide)

	if !strings.Contains(md, "# Shell Integration Guide") {
		t.Error("missing main heading")
	}
	for _, name := range []string{"Bash", "Zsh", "Fish", "Ksh"} {
		if !strings.Contains(md, "## "+name) {
			t.Errorf("missing shell section for %s", name)
		}
	}
	if !strings.Contains(md, "```sh") {
		t.Error("missing shell code blocks")
	}
}

// ---------------------------------------------------------------------------
// Man page tests
// ---------------------------------------------------------------------------

func TestManPageAllPagesGenerated(t *testing.T) {
	pages := dcAllManPages()
	if len(pages) != 5 {
		t.Errorf("man page count = %d, want 5", len(pages))
	}

	names := make(map[string]bool)
	for _, mp := range pages {
		key := mp.Name + "." + mp.Section
		names[key] = true
	}

	expected := []string{
		"prompt-pulse.1",
		"prompt-pulse-daemon.1",
		"prompt-pulse-banner.1",
		"prompt-pulse-tui.1",
		"prompt-pulse.toml.5",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing man page: %s", name)
		}
	}
}

func TestManPageRoffFormat(t *testing.T) {
	pages := dcAllManPages()
	for _, mp := range pages {
		roff := dcRenderManRoff(mp)

		// Check required roff directives
		if !strings.Contains(roff, ".TH ") {
			t.Errorf("man page %s(%s) missing .TH header", mp.Name, mp.Section)
		}
		if !strings.Contains(roff, ".SH NAME") {
			t.Errorf("man page %s(%s) missing .SH NAME", mp.Name, mp.Section)
		}
		if !strings.Contains(roff, ".SH DESCRIPTION") {
			t.Errorf("man page %s(%s) missing .SH DESCRIPTION", mp.Name, mp.Section)
		}

		// Verify the name appears in the .TH line
		upperName := strings.ToUpper(mp.Name)
		if !strings.Contains(roff, upperName) {
			t.Errorf("man page %s(%s) .TH missing uppercase name", mp.Name, mp.Section)
		}
	}
}

func TestManPageSectionsPresent(t *testing.T) {
	pages := dcAllManPages()
	for _, mp := range pages {
		if mp.ShortDesc == "" {
			t.Errorf("man page %s(%s) has empty ShortDesc", mp.Name, mp.Section)
		}
		if mp.Description == "" {
			t.Errorf("man page %s(%s) has empty Description", mp.Name, mp.Section)
		}
		if mp.SeeAlso == "" {
			t.Errorf("man page %s(%s) has empty SeeAlso", mp.Name, mp.Section)
		}
	}
}

func TestManPageByName(t *testing.T) {
	mp := dcGenerateManPage("prompt-pulse", "1")
	if mp.Name != "prompt-pulse" {
		t.Errorf("Name = %q, want prompt-pulse", mp.Name)
	}
	if mp.Section != "1" {
		t.Errorf("Section = %q, want 1", mp.Section)
	}
}

func TestManPageUnknownCommand(t *testing.T) {
	mp := dcGenerateManPage("nonexistent", "9")
	if mp.ShortDesc != "unknown command" {
		t.Errorf("ShortDesc = %q, want 'unknown command'", mp.ShortDesc)
	}
}

func TestRenderManMarkdown(t *testing.T) {
	mp := dcManPromptPulse()
	md := dcRenderManMarkdown(mp)

	if !strings.Contains(md, "# prompt-pulse(1)") {
		t.Error("missing man page heading")
	}
	if !strings.Contains(md, "## NAME") {
		t.Error("missing NAME section")
	}
	if !strings.Contains(md, "## SYNOPSIS") {
		t.Error("missing SYNOPSIS section")
	}
	if !strings.Contains(md, "## DESCRIPTION") {
		t.Error("missing DESCRIPTION section")
	}
	if !strings.Contains(md, "## SEE ALSO") {
		t.Error("missing SEE ALSO section")
	}
}

func TestManPageConfigSection5(t *testing.T) {
	mp := dcGenerateManPage("prompt-pulse.toml", "5")
	if mp.Section != "5" {
		t.Errorf("Section = %q, want 5", mp.Section)
	}
	if !strings.Contains(mp.Description, "TOML") {
		t.Error("config man page description should mention TOML")
	}
}

// ---------------------------------------------------------------------------
// Changelog tests
// ---------------------------------------------------------------------------

func TestChangelogV2Present(t *testing.T) {
	cl := dcGenerateV2Changelog()
	if len(cl.Releases) == 0 {
		t.Fatal("no releases in changelog")
	}

	r := cl.Releases[0]
	if r.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", r.Version)
	}
}

func TestChangelogDateFormat(t *testing.T) {
	cl := dcGenerateV2Changelog()
	r := cl.Releases[0]

	// YYYY-MM-DD format
	if len(r.Date) != 10 {
		t.Errorf("Date = %q, want YYYY-MM-DD format (10 chars)", r.Date)
	}
	if r.Date[4] != '-' || r.Date[7] != '-' {
		t.Errorf("Date = %q, wrong separator positions", r.Date)
	}
}

func TestChangelogAllCategories(t *testing.T) {
	cl := dcGenerateV2Changelog()
	r := cl.Releases[0]

	if len(r.Added) == 0 {
		t.Error("changelog has no Added entries")
	}
	if len(r.Changed) == 0 {
		t.Error("changelog has no Changed entries")
	}
	if len(r.Fixed) == 0 {
		t.Error("changelog has no Fixed entries")
	}
	if len(r.Removed) == 0 {
		t.Error("changelog has no Removed entries")
	}
}

func TestChangelogV2Highlights(t *testing.T) {
	cl := dcGenerateV2Changelog()
	r := cl.Releases[0]

	// Check key features are mentioned
	addedStr := strings.Join(r.Added, " ")
	keywords := []string{"Bubbletea", "Cassowary", "theme", "Ksh", "Kitty"}
	for _, kw := range keywords {
		if !strings.Contains(addedStr, kw) {
			t.Errorf("Added section missing keyword: %s", kw)
		}
	}
}

func TestRenderChangelogMarkdown(t *testing.T) {
	cl := dcGenerateV2Changelog()
	md := dcRenderChangelogMarkdown(cl)

	if !strings.Contains(md, "# Changelog") {
		t.Error("missing main heading")
	}
	if !strings.Contains(md, "Keep a Changelog") {
		t.Error("missing Keep a Changelog reference")
	}
	if !strings.Contains(md, "## [2.0.0]") {
		t.Error("missing v2.0.0 version heading")
	}
	if !strings.Contains(md, "### Added") {
		t.Error("missing Added subsection")
	}
	if !strings.Contains(md, "### Changed") {
		t.Error("missing Changed subsection")
	}
	if !strings.Contains(md, "### Fixed") {
		t.Error("missing Fixed subsection")
	}
	if !strings.Contains(md, "### Removed") {
		t.Error("missing Removed subsection")
	}
}

// ---------------------------------------------------------------------------
// Output validation tests
// ---------------------------------------------------------------------------

func TestGeneratedMarkdownHasProperHeadings(t *testing.T) {
	doc := dcGenerateArchDoc()
	md := dcRenderArchMarkdown(doc)

	// Check for sequential heading levels (no # followed by ###)
	lines := strings.Split(md, "\n")
	lastLevel := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			level := 0
			for _, c := range line {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			if level > lastLevel+1 && lastLevel > 0 {
				t.Errorf("heading level jump from %d to %d: %s", lastLevel, level, line)
			}
			lastLevel = level
		}
	}
}

func TestGeneratedMarkdownHasProperCodeBlocks(t *testing.T) {
	ref := dcGenerateConfigRef()
	md := dcRenderConfigMarkdown(ref)

	openCount := strings.Count(md, "```toml")
	closeCount := strings.Count(md, "```\n")

	// Every opening ``` should have a closing ```
	// Note: closeCount includes both opening+lang and bare closing
	// openCount should match a subset of closing markers
	if openCount == 0 {
		t.Error("no TOML code blocks found")
	}
	if closeCount < openCount {
		t.Errorf("mismatched code blocks: %d opening, %d closing", openCount, closeCount)
	}
}

// ---------------------------------------------------------------------------
// Edge case tests
// ---------------------------------------------------------------------------

func TestEmptySection(t *testing.T) {
	g := New("/tmp/docs")
	g.Add("Empty", "empty", "", 1)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}
	if !strings.Contains(doc, "## Empty") {
		t.Error("empty section should still have heading")
	}
}

func TestVeryLongDescription(t *testing.T) {
	long := strings.Repeat("This is a very long description. ", 100)
	g := New("/tmp/docs")
	g.Add("Long", "long", long, 1)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}
	if !strings.Contains(doc, long) {
		t.Error("long description not preserved")
	}
}

func TestSpecialCharactersInContent(t *testing.T) {
	content := "Contains <html> & \"quotes\" and 'apostrophes' and `backticks`"
	g := New("/tmp/docs")
	g.Add("Special", "special", content, 1)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}
	if !strings.Contains(doc, content) {
		t.Error("special characters not preserved")
	}
}

func TestUnicodeInContent(t *testing.T) {
	content := "Unicode support: \u2603 \u2764 \u2713 \u2717 \u00e9\u00e8\u00ea \u4e16\u754c"
	g := New("/tmp/docs")
	g.Add("Unicode", "unicode", content, 1)

	doc, err := g.GenerateSingle()
	if err != nil {
		t.Fatalf("GenerateSingle() error: %v", err)
	}
	if !strings.Contains(doc, content) {
		t.Error("unicode content not preserved")
	}
}

func TestRoffOutputContainsNoMarkdown(t *testing.T) {
	mp := dcManPromptPulse()
	roff := dcRenderManRoff(mp)

	// Roff should not contain markdown-style headings outside of
	// literal example blocks (.nf/.fi). Shell comments like "# comment"
	// inside examples are not markdown headings.
	inExample := false
	for _, line := range strings.Split(roff, "\n") {
		if line == ".nf" {
			inExample = true
			continue
		}
		if line == ".fi" {
			inExample = false
			continue
		}
		if inExample {
			continue
		}
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
			t.Errorf("roff output contains markdown heading outside examples: %s", line)
		}
	}
}

func TestGenerateRoffFormat(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)
	g.Format = "roff"
	g.Add("Test", "test", "Content", 1)

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	path := filepath.Join(dir, "test.1")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("roff file not created at %s: %v", path, err)
	}
}

