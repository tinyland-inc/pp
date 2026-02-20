package docs

import (
	"fmt"
	"strings"
)

// ArchDoc holds the generated architecture documentation.
type ArchDoc struct {
	// Packages lists every package in the project with metadata.
	Packages []PackageInfo

	// Diagram is an ASCII or Mermaid dependency diagram.
	Diagram string

	// Layers groups packages into architectural layers.
	Layers []LayerInfo
}

// PackageInfo describes a single Go package in the project.
type PackageInfo struct {
	// Name is the short package name.
	Name string

	// Path is the import path relative to the module root.
	Path string

	// Description summarizes what the package does.
	Description string

	// Dependencies lists other prompt-pulse packages this package imports.
	Dependencies []string

	// ExportedTypes lists key exported type names.
	ExportedTypes []string
}

// LayerInfo groups related packages into an architectural layer.
type LayerInfo struct {
	// Name is the layer name (e.g., "Core", "Rendering").
	Name string

	// Packages lists package names in this layer.
	Packages []string

	// Description summarizes the layer's responsibility.
	Description string
}

// dcGenerateArchDoc builds the full architecture documentation from embedded knowledge.
func dcGenerateArchDoc() *ArchDoc {
	packages := dcAllPackages()
	layers := dcAllLayers()
	diagram := dcArchDiagram()

	return &ArchDoc{
		Packages: packages,
		Layers:   layers,
		Diagram:  diagram,
	}
}

// dcRenderArchMarkdown renders an ArchDoc as a Markdown document.
func dcRenderArchMarkdown(doc *ArchDoc) string {
	var b strings.Builder

	b.WriteString("# Architecture\n\n")
	b.WriteString("prompt-pulse v2 is organized into packages across architectural layers.\n")
	b.WriteString("The interactive TUI is provided by the separate prompt-pulse-tui Rust binary.\n\n")

	// Layers overview
	b.WriteString("## Layers\n\n")
	for _, l := range doc.Layers {
		b.WriteString(fmt.Sprintf("### %s\n\n", l.Name))
		b.WriteString(l.Description + "\n\n")
		b.WriteString("Packages: ")
		b.WriteString(strings.Join(l.Packages, ", "))
		b.WriteString("\n\n")
	}

	// Dependency diagram
	b.WriteString("## Dependency Diagram\n\n")
	b.WriteString("```\n")
	b.WriteString(doc.Diagram)
	b.WriteString("\n```\n\n")

	// Package reference
	b.WriteString("## Package Reference\n\n")
	for _, p := range doc.Packages {
		b.WriteString(fmt.Sprintf("### `%s`\n\n", p.Path))
		b.WriteString(p.Description + "\n\n")

		if len(p.ExportedTypes) > 0 {
			b.WriteString("**Exported types:** ")
			b.WriteString(strings.Join(p.ExportedTypes, ", "))
			b.WriteString("\n\n")
		}

		if len(p.Dependencies) > 0 {
			b.WriteString("**Dependencies:** ")
			b.WriteString(strings.Join(p.Dependencies, ", "))
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

// dcAllPackages returns metadata for all 29 packages.
func dcAllPackages() []PackageInfo {
	return []PackageInfo{
		// Core layer
		{
			Name:          "layout",
			Path:          "pkg/layout",
			Description:   "Cassowary constraint-based layout engine for terminal widget positioning.",
			Dependencies:  []string{"terminal"},
			ExportedTypes: []string{"Engine", "Constraint", "Node", "Rect"},
		},
		{
			Name:          "terminal",
			Path:          "pkg/terminal",
			Description:   "Terminal capability detection, size queries, and protocol negotiation.",
			Dependencies:  nil,
			ExportedTypes: []string{"Info", "Capabilities", "Size"},
		},
		{
			Name:          "config",
			Path:          "pkg/config",
			Description:   "TOML configuration loading with defaults, environment overrides, and layout presets.",
			Dependencies:  nil,
			ExportedTypes: []string{"Config", "GeneralConfig", "LayoutConfig", "CollectorsConfig", "ImageConfig", "ThemeConfig", "ShellConfig", "BannerConfig"},
		},
		{
			Name:          "app",
			Path:          "pkg/app",
			Description:   "Application lifecycle and dependency wiring for all prompt-pulse modes.",
			Dependencies:  []string{"config", "terminal", "layout", "daemon", "banner", "shell"},
			ExportedTypes: []string{"App", "Mode"},
		},

		// Rendering layer
		{
			Name:          "image",
			Path:          "pkg/image",
			Description:   "Multi-protocol image rendering: Kitty Unicode placeholders, iTerm2 inline, Sixel, and half-blocks.",
			Dependencies:  []string{"terminal"},
			ExportedTypes: []string{"Renderer", "Protocol", "RenderResult"},
		},
		{
			Name:          "components",
			Path:          "pkg/components",
			Description:   "Reusable Bubbletea components: sparklines, gauges, tables, and bordered panels.",
			Dependencies:  []string{"theme"},
			ExportedTypes: []string{"Sparkline", "Gauge", "Table", "Panel"},
		},
		{
			Name:          "theme",
			Path:          "pkg/theme",
			Description:   "Named color themes with 6 built-in palettes: default, gruvbox, nord, catppuccin, dracula, tokyo-night.",
			Dependencies:  nil,
			ExportedTypes: []string{"Theme", "Palette", "Colors"},
		},

		// Data layer
		{
			Name:          "collectors/tailscale",
			Path:          "pkg/collectors/tailscale",
			Description:   "Tailscale status collector: peer list, exit nodes, network health.",
			Dependencies:  []string{"data"},
			ExportedTypes: []string{"Collector", "Status", "Peer"},
		},
		{
			Name:          "collectors/k8s",
			Path:          "pkg/collectors/k8s",
			Description:   "Kubernetes status collector: deployments, pods, nodes across contexts.",
			Dependencies:  []string{"data"},
			ExportedTypes: []string{"Collector", "ClusterStatus", "DeploymentInfo"},
		},
		{
			Name:          "collectors/claude",
			Path:          "pkg/collectors/claude",
			Description:   "Claude API usage collector: token counts, costs, rate limits per account.",
			Dependencies:  []string{"data"},
			ExportedTypes: []string{"Collector", "Usage", "AccountUsage"},
		},
		{
			Name:          "collectors/billing",
			Path:          "pkg/collectors/billing",
			Description:   "Cloud billing collector: Civo and DigitalOcean spend tracking.",
			Dependencies:  []string{"data"},
			ExportedTypes: []string{"Collector", "BillingSummary", "ProviderCost"},
		},
		{
			Name:          "collectors/sysmetrics",
			Path:          "pkg/collectors/sysmetrics",
			Description:   "System metrics collector: CPU, memory, disk, GPU, network via gopsutil.",
			Dependencies:  []string{"data", "sysinfo"},
			ExportedTypes: []string{"Collector", "Metrics", "CPUInfo", "MemInfo", "DiskInfo"},
		},
		{
			Name:          "data",
			Path:          "pkg/data",
			Description:   "Data pipeline: time-series storage, collector interface, and aggregation.",
			Dependencies:  nil,
			ExportedTypes: []string{"Store", "Collector", "TimeSeries", "DataPoint"},
		},
		{
			Name:          "cache",
			Path:          "pkg/cache",
			Description:   "Disk and memory caching with TTL expiry and size limits for images and API responses.",
			Dependencies:  nil,
			ExportedTypes: []string{"Cache", "Entry", "Options"},
		},

		// Shell layer
		{
			Name:          "shell",
			Path:          "pkg/shell",
			Description:   "Shell integration for Bash, Zsh, Fish, and Ksh: hooks, keybindings, completions.",
			Dependencies:  []string{"config"},
			ExportedTypes: []string{"Integration", "ShellType", "Hook"},
		},
		{
			Name:          "starship",
			Path:          "pkg/starship",
			Description:   "Starship prompt segment generation with prompt-pulse data injection.",
			Dependencies:  []string{"data"},
			ExportedTypes: []string{"Segment", "SegmentConfig"},
		},
		{
			Name:          "banner",
			Path:          "pkg/banner",
			Description:   "Terminal banner renderer with adaptive width modes: compact, standard, wide, ultra-wide.",
			Dependencies:  []string{"image", "data", "theme", "config"},
			ExportedTypes: []string{"Renderer", "Mode"},
		},

		// Integration layer
		{
			Name:          "emacs",
			Path:          "pkg/emacs",
			Description:   "Emacs integration via elisp helpers and socket-based data queries.",
			Dependencies:  []string{"daemon"},
			ExportedTypes: []string{"Client", "ElispConfig"},
		},
		{
			Name:          "daemon",
			Path:          "pkg/daemon",
			Description:   "Background daemon with Unix socket IPC, periodic data collection, and client API.",
			Dependencies:  []string{"data", "config", "cache"},
			ExportedTypes: []string{"Daemon", "Client", "Request", "Response"},
		},

		// Testing layer
		{
			Name:          "perf",
			Path:          "pkg/perf",
			Description:   "Performance benchmarking: render latency, memory allocation, and throughput tests.",
			Dependencies:  nil,
			ExportedTypes: []string{"Benchmark", "Result", "Report"},
		},
		{
			Name:          "termtest",
			Path:          "pkg/termtest",
			Description:   "Terminal test harness: virtual terminal emulation for render testing.",
			Dependencies:  []string{"terminal"},
			ExportedTypes: []string{"VTerm", "Capture"},
		},
		{
			Name:          "shelltest",
			Path:          "pkg/shelltest",
			Description:   "Shell integration test harness: script execution and output validation.",
			Dependencies:  []string{"shell"},
			ExportedTypes: []string{"Runner", "ScriptResult"},
		},
		{
			Name:          "inttest",
			Path:          "pkg/inttest",
			Description:   "Integration test utilities: daemon lifecycle, end-to-end test helpers.",
			Dependencies:  []string{"daemon"},
			ExportedTypes: []string{"Harness", "DaemonFixture"},
		},

		// Platform layer
		{
			Name:          "platform",
			Path:          "pkg/platform",
			Description:   "Cross-platform abstractions for macOS and Linux: process, filesystem, display.",
			Dependencies:  nil,
			ExportedTypes: []string{"OS", "DisplayInfo"},
		},
		{
			Name:          "sysinfo",
			Path:          "pkg/sysinfo",
			Description:   "System information queries: hardware, OS version, GPU, disk (APFS-aware).",
			Dependencies:  []string{"platform"},
			ExportedTypes: []string{"Info", "GPUInfo", "DiskUsage"},
		},

		// Packaging layer
		{
			Name:          "nixpkg",
			Path:          "pkg/nixpkg",
			Description:   "Nix packaging: buildGoModule derivation, overlay, dev shell, and vendor hash generation.",
			Dependencies:  nil,
			ExportedTypes: []string{"PackageMeta", "Derivation", "Overlay", "DevShell"},
		},
		{
			Name:          "homebrew",
			Path:          "pkg/homebrew",
			Description:   "Homebrew formula generation and tap management for macOS distribution.",
			Dependencies:  nil,
			ExportedTypes: []string{"Formula", "Tap"},
		},
		{
			Name:          "migrate",
			Path:          "pkg/migrate",
			Description:   "v1-to-v2 config migration: parse flat format, transform to nested TOML, backup originals.",
			Dependencies:  nil,
			ExportedTypes: []string{"Migrator", "V1Config", "MigrationResult"},
		},
		{
			Name:          "docs",
			Path:          "pkg/docs",
			Description:   "Documentation generator: architecture docs, config reference, shell guides, man pages, changelog.",
			Dependencies:  nil,
			ExportedTypes: []string{"DocGenerator", "Section", "ArchDoc", "ConfigRef", "ShellGuide", "ManPage", "Changelog"},
		},
	}
}

// dcAllLayers returns the 10 architectural layers.
func dcAllLayers() []LayerInfo {
	return []LayerInfo{
		{
			Name:        "Core",
			Packages:    []string{"layout", "terminal", "config", "app"},
			Description: "Foundational packages for layout computation, terminal interaction, configuration, and application lifecycle.",
		},
		{
			Name:        "Rendering",
			Packages:    []string{"image", "components", "theme"},
			Description: "Visual rendering: multi-protocol image output, reusable UI components, and color theme palettes.",
		},
		{
			Name:        "Data",
			Packages:    []string{"collectors/tailscale", "collectors/k8s", "collectors/claude", "collectors/billing", "collectors/sysmetrics", "data", "cache"},
			Description: "Data collection, storage, and caching. Each collector fetches from a specific data source on a configurable interval.",
		},
		{
			Name:        "Shell",
			Packages:    []string{"shell", "starship", "banner"},
			Description: "Shell integration hooks, Starship prompt segments, and terminal banner rendering.",
		},
		{
			Name:        "Integration",
			Packages:    []string{"emacs", "daemon"},
			Description: "External tool integration: Emacs elisp bridge and background daemon with Unix socket IPC.",
		},
		{
			Name:        "Testing",
			Packages:    []string{"perf", "termtest", "shelltest", "inttest"},
			Description: "Test infrastructure: performance benchmarks, terminal emulation, shell script testing, and integration test harnesses.",
		},
		{
			Name:        "Platform",
			Packages:    []string{"platform", "sysinfo"},
			Description: "Cross-platform abstractions for macOS and Linux, including APFS-aware disk reporting.",
		},
		{
			Name:        "Packaging",
			Packages:    []string{"nixpkg", "homebrew", "migrate", "docs"},
			Description: "Distribution packaging (Nix, Homebrew), configuration migration, and documentation generation.",
		},
	}
}

// dcArchDiagram returns an ASCII dependency diagram.
func dcArchDiagram() string {
	return `                    +----------+
                    |   app    |
                    +----+-----+
                         |
         +---------------+----------------+
         |               |                |
    +----v-----+   +-----v-----+    +-----v-----+
    |  banner  |   |  daemon   |    |   shell   |
    +----+-----+   +-----------+    +-----+-----+
         |                                |
    +----v-----+                          |
    |  image   |                          |
    +----------+                          |
                                          |
    +----v----+    +-----+-----+    +-----v-----+
    |  theme  |    |   data    |    |  config   |
    +---------+    +-----+-----+    +-----------+
                         |
              +----------+----------+
              |          |          |
         +----v---+ +----v---+ +---v----+
         |tailscl | | claude | | sysmet |
         +--------+ +--------+ +--------+
         +--------+ +--------+
         |  k8s   | |billing |
         +--------+ +--------+

    TUI: prompt-pulse-tui (Rust/ratatui) — separate binary`
}
