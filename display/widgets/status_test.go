package widgets

import (
	"strings"
	"testing"
)

func TestRenderStatus_OK(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusOK,
		Text:     "All systems operational",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CF") {
		t.Error("expected green dot icon")
	}
	if !strings.Contains(result, "All systems operational") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_Warning(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusWarning,
		Text:     "High latency",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CF") {
		t.Error("expected yellow dot icon")
	}
	if !strings.Contains(result, "High latency") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_Critical(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusCritical,
		Text:     "Service down",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CF") {
		t.Error("expected red dot icon")
	}
	if !strings.Contains(result, "Service down") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_Unknown(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusUnknown,
		Text:     "No data",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CB") {
		t.Error("expected gray outline icon")
	}
	if !strings.Contains(result, "No data") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_Pending(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusPending,
		Text:     "Checking...",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CC") {
		t.Error("expected dotted circle icon")
	}
	if !strings.Contains(result, "Checking...") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_NoIcon(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusOK,
		Text:     "healthy",
		ShowIcon: false,
	})
	if strings.Contains(result, "\u25CF") {
		t.Error("expected no dot icon when ShowIcon is false")
	}
	if !strings.Contains(result, "healthy") {
		t.Error("expected status text")
	}
}

func TestRenderStatus_EmptyText(t *testing.T) {
	result := RenderStatus(StatusConfig{
		Level:    StatusCritical,
		Text:     "",
		ShowIcon: true,
	})
	if !strings.Contains(result, "\u25CF") {
		t.Error("expected icon even with empty text")
	}
	// Should be just the icon, no trailing space.
	if strings.HasSuffix(result, " ") {
		t.Error("expected no trailing space when text is empty")
	}
}

func TestRenderStatusFromString(t *testing.T) {
	tests := []struct {
		input    string
		wantIcon string
	}{
		{"ok", "\u25CF"},
		{"healthy", "\u25CF"},
		{"Ready", "\u25CF"},
		{"warning", "\u25CF"},
		{"degraded", "\u25CF"},
		{"error", "\u25CF"},
		{"critical", "\u25CF"},
		{"auth_failed", "\u25CF"},
		{"NotReady", "\u25CF"},
		{"pending", "\u25CC"},
		{"limited", "\u25CC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := RenderStatusFromString(tt.input)
			if !strings.Contains(result, tt.wantIcon) {
				t.Errorf("RenderStatusFromString(%q) missing expected icon %q, got %q", tt.input, tt.wantIcon, result)
			}
			if !strings.Contains(result, tt.input) {
				t.Errorf("RenderStatusFromString(%q) missing input text", tt.input)
			}
		})
	}
}

func TestRenderStatusFromString_Unknown(t *testing.T) {
	result := RenderStatusFromString("something_else")
	if !strings.Contains(result, "\u25CB") {
		t.Error("expected unknown icon (outline circle) for unrecognized string")
	}
	if !strings.Contains(result, "something_else") {
		t.Error("expected original text preserved")
	}
}

func TestStatusLevelFromString(t *testing.T) {
	tests := []struct {
		input string
		want  StatusLevel
	}{
		{"ok", StatusOK},
		{"healthy", StatusOK},
		{"Ready", StatusOK},
		{"warning", StatusWarning},
		{"degraded", StatusWarning},
		{"error", StatusCritical},
		{"critical", StatusCritical},
		{"auth_failed", StatusCritical},
		{"NotReady", StatusCritical},
		{"pending", StatusPending},
		{"limited", StatusPending},
		{"unknown_value", StatusUnknown},
		{"", StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StatusLevelFromString(tt.input)
			if got != tt.want {
				t.Errorf("StatusLevelFromString(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestStatusIcons_AllLevels(t *testing.T) {
	levels := []StatusLevel{StatusOK, StatusWarning, StatusCritical, StatusUnknown, StatusPending}
	for _, level := range levels {
		icon, ok := statusIcons[level]
		if !ok {
			t.Errorf("missing icon for StatusLevel %d", level)
		}
		if icon == "" {
			t.Errorf("empty icon for StatusLevel %d", level)
		}
	}
}

func TestStatusColors_AllLevels(t *testing.T) {
	levels := []StatusLevel{StatusOK, StatusWarning, StatusCritical, StatusUnknown, StatusPending}
	for _, level := range levels {
		color, ok := statusColors[level]
		if !ok {
			t.Errorf("missing color for StatusLevel %d", level)
		}
		if string(color) == "" {
			t.Errorf("empty color for StatusLevel %d", level)
		}
	}
}
