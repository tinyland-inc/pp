package banner

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestDefaultLayoutConfig(t *testing.T) {
	cfg := DefaultLayoutConfig()

	if cfg.TermWidth != 80 {
		t.Errorf("TermWidth: got %d, want 80", cfg.TermWidth)
	}
	if cfg.TermHeight != 24 {
		t.Errorf("TermHeight: got %d, want 24", cfg.TermHeight)
	}
	if cfg.ImageCols != 22 {
		t.Errorf("ImageCols: got %d, want 22", cfg.ImageCols)
	}
	if !cfg.ShowImage {
		t.Error("ShowImage: got false, want true")
	}
	if cfg.Hostname != "" {
		t.Errorf("Hostname: got %q, want empty", cfg.Hostname)
	}
	if !cfg.ColorEnabled {
		t.Error("ColorEnabled: got false, want true")
	}
}

func TestNewLayout(t *testing.T) {
	cfg := DefaultLayoutConfig()
	l := NewLayout(cfg)
	if l == nil {
		t.Fatal("NewLayout returned nil")
	}
	if l.config.TermWidth != 80 {
		t.Errorf("config.TermWidth: got %d, want 80", l.config.TermWidth)
	}
}

func noColorConfig() LayoutConfig {
	cfg := DefaultLayoutConfig()
	cfg.ColorEnabled = false
	cfg.Hostname = "testhost"
	return cfg
}

func TestRenderHeader(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		status   string
		wantSub  string
	}{
		{
			name:     "healthy status",
			hostname: "yoga",
			status:   "healthy",
			wantSub:  "yoga :: healthy",
		},
		{
			name:     "critical status",
			hostname: "honey",
			status:   "critical",
			wantSub:  "honey :: critical",
		},
		{
			name:     "warning status",
			hostname: "dev",
			status:   "warning",
			wantSub:  "dev :: warning",
		},
		{
			name:     "empty status defaults to unknown",
			hostname: "server",
			status:   "",
			wantSub:  "server :: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := noColorConfig()
			l := NewLayout(cfg)
			got := l.renderHeader(tt.hostname, tt.status)
			if got != tt.wantSub {
				t.Errorf("renderHeader(%q, %q) = %q, want %q",
					tt.hostname, tt.status, got, tt.wantSub)
			}
		})
	}
}

func TestRenderClaudeSummary(t *testing.T) {
	tests := []struct {
		name    string
		data    *collectors.ClaudeUsage
		wantSub []string
	}{
		{
			name:    "nil data",
			data:    nil,
			wantSub: []string{"(no data)"},
		},
		{
			name:    "empty accounts",
			data:    &collectors.ClaudeUsage{Accounts: []collectors.ClaudeAccountUsage{}},
			wantSub: []string{"(no data)"},
		},
		{
			name: "subscription account with 5h and 7d",
			data: &collectors.ClaudeUsage{
				Accounts: []collectors.ClaudeAccountUsage{
					{
						Name:   "personal",
						Type:   "subscription",
						Status: "ok",
						FiveHour: &collectors.UsagePeriod{
							Utilization: 45.0,
						},
						SevenDay: &collectors.UsagePeriod{
							Utilization: 12.0,
						},
					},
				},
			},
			wantSub: []string{"personal: 45% (5h) | 12% (7d)"},
		},
		{
			name: "subscription account with 5h only",
			data: &collectors.ClaudeUsage{
				Accounts: []collectors.ClaudeAccountUsage{
					{
						Name:   "personal",
						Type:   "subscription",
						Status: "ok",
						FiveHour: &collectors.UsagePeriod{
							Utilization: 80.0,
						},
					},
				},
			},
			wantSub: []string{"personal: 80% (5h)"},
		},
		{
			name: "API account",
			data: &collectors.ClaudeUsage{
				Accounts: []collectors.ClaudeAccountUsage{
					{
						Name:   "work-api",
						Type:   "api",
						Status: "ok",
						RateLimits: &collectors.APIRateLimits{
							RequestsLimit:     4000,
							RequestsRemaining: 1660,
						},
					},
				},
			},
			wantSub: []string{"work-api: 2340/4000 req"},
		},
		{
			name: "errored account",
			data: &collectors.ClaudeUsage{
				Accounts: []collectors.ClaudeAccountUsage{
					{
						Name:   "broken",
						Type:   "subscription",
						Status: "auth_failed",
					},
				},
			},
			wantSub: []string{"broken: ERR"},
		},
		{
			name: "mixed accounts",
			data: &collectors.ClaudeUsage{
				Accounts: []collectors.ClaudeAccountUsage{
					{
						Name:   "personal",
						Type:   "subscription",
						Status: "ok",
						FiveHour: &collectors.UsagePeriod{
							Utilization: 45.0,
						},
					},
					{
						Name:   "broken",
						Type:   "api",
						Status: "rate_limited",
					},
				},
			},
			wantSub: []string{"personal: 45% (5h)", "broken: ERR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := noColorConfig()
			l := NewLayout(cfg)
			got := l.renderClaudeSummary(tt.data)
			for i, want := range tt.wantSub {
				found := false
				for _, line := range got {
					if strings.Contains(line, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("renderClaudeSummary[%d]: want substring %q in output %v",
						i, want, got)
				}
			}
		})
	}
}

func TestRenderBillingSummary(t *testing.T) {
	forecast180 := 180.0
	budget100 := 100.0
	budget200 := 200.0

	tests := []struct {
		name    string
		data    *collectors.BillingData
		wantSub []string
	}{
		{
			name:    "nil data",
			data:    nil,
			wantSub: []string{"(no data)"},
		},
		{
			name: "spend only",
			data: &collectors.BillingData{
				Total: collectors.BillingSummary{
					CurrentMonthUSD: 142.0,
				},
			},
			wantSub: []string{"$142 this month"},
		},
		{
			name: "spend with forecast",
			data: &collectors.BillingData{
				Total: collectors.BillingSummary{
					CurrentMonthUSD: 142.0,
					ForecastUSD:     &forecast180,
				},
			},
			wantSub: []string{"$142 this month", "$180 forecast"},
		},
		{
			name: "budget exceeded",
			data: &collectors.BillingData{
				Total: collectors.BillingSummary{
					CurrentMonthUSD: 142.0,
					BudgetUSD:       &budget100,
				},
			},
			wantSub: []string{"$142 this month", "OVER BUDGET"},
		},
		{
			name: "budget not exceeded",
			data: &collectors.BillingData{
				Total: collectors.BillingSummary{
					CurrentMonthUSD: 142.0,
					BudgetUSD:       &budget200,
				},
			},
			wantSub: []string{"$142 this month"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := noColorConfig()
			l := NewLayout(cfg)
			got := l.renderBillingSummary(tt.data)
			joined := strings.Join(got, "\n")
			for i, want := range tt.wantSub {
				if !strings.Contains(joined, want) {
					t.Errorf("renderBillingSummary[%d]: want substring %q in output %q",
						i, want, joined)
				}
			}
		})
	}
}

func TestRenderBillingSummaryBudgetNotExceededNoOverBudgetText(t *testing.T) {
	budget200 := 200.0
	cfg := noColorConfig()
	l := NewLayout(cfg)
	data := &collectors.BillingData{
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 142.0,
			BudgetUSD:       &budget200,
		},
	}
	got := l.renderBillingSummary(data)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "OVER BUDGET") {
		t.Errorf("should not contain OVER BUDGET when under budget, got %q", joined)
	}
}

func TestRenderInfraSummary(t *testing.T) {
	tests := []struct {
		name    string
		data    *collectors.InfraStatus
		wantSub []string
	}{
		{
			name:    "nil data",
			data:    nil,
			wantSub: []string{"(no data)"},
		},
		{
			name: "empty infra no tailscale no k8s",
			data: &collectors.InfraStatus{},
			wantSub: []string{"(no data)"},
		},
		{
			name: "tailscale data",
			data: &collectors.InfraStatus{
				Tailscale: &collectors.TailscaleStatus{
					OnlineCount: 4,
					TotalCount:  5,
				},
			},
			wantSub: []string{"ts: 4/5 online"},
		},
		{
			name: "kubernetes data",
			data: &collectors.InfraStatus{
				Kubernetes: []collectors.KubernetesCluster{
					{
						Name:   "bitter-darkness",
						Status: "healthy",
					},
				},
			},
			wantSub: []string{"k8s: bitter-darkness (healthy)"},
		},
		{
			name: "tailscale and kubernetes",
			data: &collectors.InfraStatus{
				Tailscale: &collectors.TailscaleStatus{
					OnlineCount: 3,
					TotalCount:  6,
				},
				Kubernetes: []collectors.KubernetesCluster{
					{
						Name:   "prod",
						Status: "healthy",
					},
					{
						Name:   "staging",
						Status: "degraded",
					},
				},
			},
			wantSub: []string{"ts: 3/6 online", "k8s: prod (healthy)", "k8s: staging (degraded)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := noColorConfig()
			l := NewLayout(cfg)
			got := l.renderInfraSummary(tt.data)
			joined := strings.Join(got, "\n")
			for i, want := range tt.wantSub {
				if !strings.Contains(joined, want) {
					t.Errorf("renderInfraSummary[%d]: want substring %q in output %q",
						i, want, joined)
				}
			}
		})
	}
}

func fullInfoData() InfoData {
	forecast180 := 180.0
	return InfoData{
		Claude: &collectors.ClaudeUsage{
			Accounts: []collectors.ClaudeAccountUsage{
				{
					Name:   "personal",
					Type:   "subscription",
					Status: "ok",
					FiveHour: &collectors.UsagePeriod{
						Utilization: 45.0,
					},
					SevenDay: &collectors.UsagePeriod{
						Utilization: 12.0,
					},
				},
			},
		},
		Billing: &collectors.BillingData{
			Total: collectors.BillingSummary{
				CurrentMonthUSD: 142.0,
				ForecastUSD:     &forecast180,
			},
		},
		Infra: &collectors.InfraStatus{
			Tailscale: &collectors.TailscaleStatus{
				OnlineCount: 4,
				TotalCount:  5,
			},
			Kubernetes: []collectors.KubernetesCluster{
				{
					Name:   "bitter-darkness",
					Status: "healthy",
				},
			},
		},
		StatusLevel: "healthy",
		Uptime:      "3d 14h",
	}
}

func fakeImage(width, height int) string {
	var lines []string
	row := strings.Repeat("X", width)
	for i := 0; i < height; i++ {
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

func TestRenderWithImageAndFullData(t *testing.T) {
	cfg := noColorConfig()
	cfg.ShowImage = true
	l := NewLayout(cfg)

	image := fakeImage(20, 10)
	data := fullInfoData()

	got := l.Render(image, data)
	if got == "" {
		t.Fatal("Render returned empty string")
	}

	// Should contain the separator from side-by-side.
	if !strings.Contains(got, " | ") {
		t.Error("expected side-by-side separator ' | ' in output")
	}

	// Should contain info panel content.
	for _, want := range []string{"testhost", "healthy", "Claude", "Billing", "Infrastructure"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render output missing %q", want)
		}
	}
}

func TestRenderWithoutImage(t *testing.T) {
	cfg := noColorConfig()
	cfg.ShowImage = false
	l := NewLayout(cfg)

	image := fakeImage(20, 10)
	data := fullInfoData()

	got := l.Render(image, data)
	if got == "" {
		t.Fatal("Render returned empty string")
	}

	// Should NOT contain image separator since ShowImage is false.
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		// The info panel itself should not have " | " as a side-by-side separator.
		// (It could appear in Claude data, so we check the first char is not image).
		if strings.HasPrefix(line, "XXXX") {
			t.Error("image content should not appear when ShowImage is false")
		}
	}

	// Should still contain info panel content.
	if !strings.Contains(got, "Claude") {
		t.Error("info panel content missing when ShowImage is false")
	}
}

func TestRenderWithEmptyImageContent(t *testing.T) {
	cfg := noColorConfig()
	cfg.ShowImage = true
	l := NewLayout(cfg)

	data := fullInfoData()

	got := l.Render("", data)
	if got == "" {
		t.Fatal("Render returned empty string")
	}

	// With empty image content and ShowImage true, should fall back to info only.
	if !strings.Contains(got, "Claude") {
		t.Error("info panel content missing with empty image content")
	}
}

func TestRenderWithNilDataFields(t *testing.T) {
	cfg := noColorConfig()
	cfg.ShowImage = false
	l := NewLayout(cfg)

	data := InfoData{
		StatusLevel: "unknown",
	}

	got := l.Render("", data)
	if got == "" {
		t.Fatal("Render returned empty string")
	}

	// All sections should show "(no data)".
	count := strings.Count(got, "(no data)")
	if count < 3 {
		t.Errorf("expected at least 3 '(no data)' markers, got %d in:\n%s", count, got)
	}
}

func TestComposeSideBySideAlignment(t *testing.T) {
	cfg := noColorConfig()
	cfg.ImageCols = 10
	l := NewLayout(cfg)

	image := "AAAA\nBBBB\nCCCC"
	info := "line1\nline2\nline3"

	got := l.composeSideBySide(image, info)
	lines := strings.Split(got, "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Each line should have image padded to 10 cols + " | " + info.
	for i, line := range lines {
		if !strings.Contains(line, " | ") {
			t.Errorf("line %d missing separator: %q", i, line)
		}
	}

	// First line: "AAAA      | line1" (AAAA padded to 10).
	if !strings.HasPrefix(lines[0], "AAAA") {
		t.Errorf("line 0 should start with AAAA: %q", lines[0])
	}
	if !strings.HasSuffix(lines[0], "line1") {
		t.Errorf("line 0 should end with line1: %q", lines[0])
	}
}

func TestComposeSideBySideImageTallerThanInfo(t *testing.T) {
	cfg := noColorConfig()
	cfg.ImageCols = 10
	l := NewLayout(cfg)

	image := "A\nB\nC\nD\nE"
	info := "X\nY"

	got := l.composeSideBySide(image, info)
	lines := strings.Split(got, "\n")

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (image height), got %d", len(lines))
	}

	// Lines beyond info should have empty info side.
	if !strings.Contains(lines[0], "X") {
		t.Errorf("line 0 missing info: %q", lines[0])
	}
	// Line 2+ should have separator but empty info side.
	if !strings.Contains(lines[2], " | ") {
		t.Errorf("line 2 missing separator: %q", lines[2])
	}
}

func TestComposeSideBySideInfoTallerThanImage(t *testing.T) {
	cfg := noColorConfig()
	cfg.ImageCols = 10
	l := NewLayout(cfg)

	image := "A\nB"
	info := "X\nY\nZ\nW\nV"

	got := l.composeSideBySide(image, info)
	lines := strings.Split(got, "\n")

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (info height), got %d", len(lines))
	}

	// Lines beyond image should have padded empty image side.
	if !strings.Contains(lines[4], "V") {
		t.Errorf("line 4 missing info content: %q", lines[4])
	}
}

func TestOutputFitsWithin80Columns(t *testing.T) {
	cfg := noColorConfig()
	cfg.TermWidth = 80
	cfg.ImageCols = 22
	cfg.ShowImage = true
	l := NewLayout(cfg)

	image := fakeImage(20, 10)
	data := fullInfoData()

	got := l.Render(image, data)
	lines := strings.Split(got, "\n")

	for i, line := range lines {
		// visibleLen strips ANSI codes for accurate width measurement.
		vLen := visibleLen(line)
		if vLen > 80 {
			t.Errorf("line %d exceeds 80 cols (visible len %d): %q", i, vLen, line)
		}
	}
}

func TestOutputFitsWithin24Rows(t *testing.T) {
	cfg := noColorConfig()
	cfg.TermHeight = 24
	cfg.ShowImage = true
	l := NewLayout(cfg)

	// Create a tall image to test truncation.
	image := fakeImage(20, 30)
	data := fullInfoData()

	got := l.Render(image, data)
	lines := strings.Split(got, "\n")

	if len(lines) > 24 {
		t.Errorf("output exceeds 24 rows: got %d rows", len(lines))
	}
}

func TestComposeSideBySideEmptyImage(t *testing.T) {
	cfg := noColorConfig()
	l := NewLayout(cfg)

	info := "line1\nline2\nline3"
	got := l.composeSideBySide("", info)

	// Should return just the info without separators.
	if strings.Contains(got, " | ") {
		t.Error("composeSideBySide with empty image should not have separator")
	}
	if !strings.Contains(got, "line1") {
		t.Error("composeSideBySide with empty image should contain info content")
	}
}

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"plain text", "hello", 5},
		{"empty", "", 0},
		{"ansi color", "\x1b[31mred\x1b[0m", 3},
		{"mixed", "pre\x1b[32mgreen\x1b[0mpost", 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleLen(tt.s)
			if got != tt.want {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestRenderHeaderWithColor(t *testing.T) {
	cfg := DefaultLayoutConfig()
	cfg.Hostname = "yoga"
	l := NewLayout(cfg)

	got := l.renderHeader("yoga", "healthy")
	// Should contain the hostname and status text (with ANSI escapes).
	plain := stripANSI(got)
	if !strings.Contains(plain, "yoga") {
		t.Errorf("colored header missing hostname: %q", plain)
	}
	if !strings.Contains(plain, "healthy") {
		t.Errorf("colored header missing status: %q", plain)
	}
}

func TestRenderInfoPanelIncludesAllSections(t *testing.T) {
	cfg := noColorConfig()
	l := NewLayout(cfg)
	data := fullInfoData()

	got := l.renderInfoPanel(data)

	for _, section := range []string{"Claude", "Billing", "Infrastructure", "uptime"} {
		if !strings.Contains(got, section) {
			t.Errorf("info panel missing section %q", section)
		}
	}
}

func TestRenderInfoPanelDefaultHostname(t *testing.T) {
	cfg := noColorConfig()
	cfg.Hostname = ""
	l := NewLayout(cfg)

	got := l.renderInfoPanel(InfoData{StatusLevel: "unknown"})
	if !strings.Contains(got, "localhost") {
		t.Error("info panel should default to 'localhost' when Hostname is empty")
	}
}

// stripANSI removes ANSI escape sequences from a string for plain text comparison.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
