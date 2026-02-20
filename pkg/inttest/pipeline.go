package inttest

import (
	"fmt"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/layout"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/preset"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/shell"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/theme"
)

// Pipeline executes a sequence of stages where each stage has a run and
// verify phase. Stages execute sequentially and a stage failure stops
// the pipeline.
type Pipeline struct {
	stages  []PipelineStage
	results []StageResult
}

// PipelineStage defines a named step in the pipeline with run and verify
// functions.
type PipelineStage struct {
	// Name identifies this stage.
	Name string

	// Run performs the stage's main work.
	Run func() error

	// Verify checks that the stage completed correctly.
	Verify func() error
}

// StageResult captures the outcome of a single pipeline stage.
type StageResult struct {
	// Name is the stage name.
	Name string

	// Passed is true if both Run and Verify succeeded.
	Passed bool

	// Duration is how long the stage took.
	Duration time.Duration

	// Error holds the error message if the stage failed.
	Error string
}

// itNewPipeline creates a new empty Pipeline.
func itNewPipeline() *Pipeline {
	return &Pipeline{}
}

// AddStage appends a stage to the pipeline.
func (p *Pipeline) AddStage(name string, run, verify func() error) {
	p.stages = append(p.stages, PipelineStage{
		Name:   name,
		Run:    run,
		Verify: verify,
	})
}

// Execute runs all stages sequentially, returning the results. Execution
// stops at the first stage failure.
func (p *Pipeline) Execute() ([]StageResult, error) {
	p.results = make([]StageResult, 0, len(p.stages))

	for _, stage := range p.stages {
		start := time.Now()
		result := StageResult{Name: stage.Name}

		if err := stage.Run(); err != nil {
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("run: %v", err)
			p.results = append(p.results, result)
			return p.results, fmt.Errorf("stage %q run failed: %w", stage.Name, err)
		}

		if stage.Verify != nil {
			if err := stage.Verify(); err != nil {
				result.Duration = time.Since(start)
				result.Error = fmt.Sprintf("verify: %v", err)
				p.results = append(p.results, result)
				return p.results, fmt.Errorf("stage %q verify failed: %w", stage.Name, err)
			}
		}

		result.Duration = time.Since(start)
		result.Passed = true
		p.results = append(p.results, result)
	}

	return p.results, nil
}

// itStageConfig returns a pipeline stage that loads config and validates
// all sections parse correctly.
func itStageConfig() (func() error, func() error) {
	var cfg *config.Config

	run := func() error {
		var err error
		cfg, err = config.LoadFromReader(strings.NewReader(itMockConfig()))
		return err
	}

	verify := func() error {
		if cfg == nil {
			return fmt.Errorf("config is nil after load")
		}
		if cfg.General.LogLevel == "" {
			return fmt.Errorf("general.log_level is empty")
		}
		if cfg.Layout.Preset == "" {
			return fmt.Errorf("layout.preset is empty")
		}
		if cfg.Theme.Name == "" {
			return fmt.Errorf("theme.name is empty")
		}
		if cfg.Banner.CompactMaxWidth <= 0 {
			return fmt.Errorf("banner.compact_max_width is zero")
		}
		return nil
	}

	return run, verify
}

// itStageCollectors returns a pipeline stage that verifies collector data
// structures can be created and are non-nil. Since we cannot import
// collector packages with external dependencies, this uses the mock data
// generators and validates their structure.
func itStageCollectors() (func() error, func() error) {
	var mockResults map[string]map[string]any

	run := func() error {
		mockResults = map[string]map[string]any{
			"claude":    itMockClaudeData(),
			"billing":   itMockBillingData(),
			"tailscale": itMockTailscaleData(),
			"k8s":       itMockK8sData(),
			"sysmetrics": itMockSysMetrics(),
		}
		return nil
	}

	verify := func() error {
		for name, data := range mockResults {
			if data == nil {
				return fmt.Errorf("collector %q returned nil data", name)
			}
			if len(data) == 0 {
				return fmt.Errorf("collector %q returned empty data", name)
			}
		}
		return nil
	}

	return run, verify
}

// itStageCache returns a pipeline stage that writes to cache, reads back,
// and verifies the round-trip.
func itStageCache() (func() error, func() error) {
	var cacheDir string
	var readBack string

	run := func() error {
		dir, cleanup, err := itTempDir("inttest-cache")
		if err != nil {
			return err
		}
		cacheDir = dir
		// Cleanup handled externally; store for verify.
		_ = cleanup

		store, err := itNewCacheStore(cacheDir)
		if err != nil {
			return fmt.Errorf("create cache store: %w", err)
		}
		defer store.Close()

		if err := store.PutString("test-key", "test-value"); err != nil {
			return fmt.Errorf("cache put: %w", err)
		}

		val, ok := store.GetString("test-key")
		if !ok {
			return fmt.Errorf("cache get returned not-found")
		}
		readBack = val
		return nil
	}

	verify := func() error {
		if readBack != "test-value" {
			return fmt.Errorf("cache round-trip: got %q, want %q", readBack, "test-value")
		}
		return nil
	}

	return run, verify
}

// itStageBanner returns a pipeline stage that renders a banner with mock
// data and verifies the output has expected properties.
func itStageBanner() (func() error, func() error) {
	var output string

	run := func() error {
		data := banner.BannerData{
			Widgets: itMockBannerWidgets(),
		}
		output = banner.Render(data, banner.Standard)
		return nil
	}

	verify := func() error {
		if output == "" {
			return fmt.Errorf("banner render returned empty string")
		}
		lines := strings.Split(output, "\n")
		if len(lines) < 5 {
			return fmt.Errorf("banner has %d lines, expected at least 5", len(lines))
		}
		return nil
	}

	return run, verify
}

// itStageShell returns a pipeline stage that generates shell scripts for
// all four shells and validates syntax patterns.
func itStageShell() (func() error, func() error) {
	var scripts map[shell.ShellType]string

	run := func() error {
		opts := shell.Options{
			BinaryPath:       "prompt-pulse",
			ShowBanner:       true,
			DaemonAutoStart:  true,
			EnableCompletions: true,
		}

		scripts = make(map[shell.ShellType]string)
		for _, sh := range []shell.ShellType{shell.Bash, shell.Zsh, shell.Fish, shell.Ksh} {
			scripts[sh] = shell.Generate(sh, opts)
		}
		return nil
	}

	verify := func() error {
		for sh, script := range scripts {
			if script == "" {
				return fmt.Errorf("shell %q generated empty script", sh)
			}
			if !strings.Contains(script, "prompt-pulse") {
				return fmt.Errorf("shell %q script missing binary name", sh)
			}
		}
		return nil
	}

	return run, verify
}

// itStageLayout returns a pipeline stage that validates the layout solver
// with multiple constraint types.
func itStageLayout() (func() error, func() error) {
	var rects []layout.Rect

	run := func() error {
		l := layout.NewLayout(layout.Horizontal,
			layout.Length{Value: 20},
			layout.Fill{Weight: 1},
			layout.Percentage{Value: 30},
		)
		area := layout.Rect{X: 0, Y: 0, Width: 120, Height: 35}
		rects = l.Split(area)
		return nil
	}

	verify := func() error {
		if len(rects) != 3 {
			return fmt.Errorf("layout split returned %d rects, want 3", len(rects))
		}
		for i, r := range rects {
			if r.Width <= 0 {
				return fmt.Errorf("rect[%d] has non-positive width: %d", i, r.Width)
			}
			if r.Height != 35 {
				return fmt.Errorf("rect[%d] has height %d, want 35", i, r.Height)
			}
		}
		return nil
	}

	return run, verify
}

// itStageTheme returns a pipeline stage that verifies all themes can be
// loaded and applied.
func itStageTheme() (func() error, func() error) {
	var themes []theme.Theme

	run := func() error {
		names := theme.Names()
		themes = make([]theme.Theme, 0, len(names))
		for _, name := range names {
			themes = append(themes, theme.Get(name))
		}
		return nil
	}

	verify := func() error {
		if len(themes) == 0 {
			return fmt.Errorf("no themes registered")
		}
		for _, t := range themes {
			if t.Name == "" {
				return fmt.Errorf("theme has empty name")
			}
			if t.Background == "" {
				return fmt.Errorf("theme %q has empty background", t.Name)
			}
			if t.Foreground == "" {
				return fmt.Errorf("theme %q has empty foreground", t.Name)
			}
		}
		return nil
	}

	return run, verify
}

// itStagePreset returns a pipeline stage that verifies all presets can be
// loaded and resolved.
func itStagePreset() (func() error, func() error) {
	var resolved []preset.ResolvedCell

	run := func() error {
		p := preset.Get("dashboard")
		resolved = preset.Resolve(p, 120, 35)
		return nil
	}

	verify := func() error {
		if len(resolved) == 0 {
			return fmt.Errorf("preset resolve returned no cells")
		}
		for _, cell := range resolved {
			if cell.WidgetID == "" {
				return fmt.Errorf("resolved cell has empty widget ID")
			}
			if cell.W <= 0 || cell.H <= 0 {
				return fmt.Errorf("resolved cell %q has zero dimensions: %dx%d",
					cell.WidgetID, cell.W, cell.H)
			}
		}
		return nil
	}

	return run, verify
}

// itBuildFullPipeline creates a pipeline with all standard stages.
func itBuildFullPipeline() *Pipeline {
	p := itNewPipeline()

	cfgRun, cfgVerify := itStageConfig()
	p.AddStage("config", cfgRun, cfgVerify)

	collRun, collVerify := itStageCollectors()
	p.AddStage("collectors", collRun, collVerify)

	cacheRun, cacheVerify := itStageCache()
	p.AddStage("cache", cacheRun, cacheVerify)

	themeRun, themeVerify := itStageTheme()
	p.AddStage("theme", themeRun, themeVerify)

	presetRun, presetVerify := itStagePreset()
	p.AddStage("preset", presetRun, presetVerify)

	layoutRun, layoutVerify := itStageLayout()
	p.AddStage("layout", layoutRun, layoutVerify)

	bannerRun, bannerVerify := itStageBanner()
	p.AddStage("banner", bannerRun, bannerVerify)

	shellRun, shellVerify := itStageShell()
	p.AddStage("shell", shellRun, shellVerify)

	return p
}
