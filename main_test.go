package main

import (
	"testing"
	"time"
)

func TestParseDuration_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"15m", 15 * time.Minute},
		{"1h", 1 * time.Hour},
		{"30s", 30 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"2h30m", 2*time.Hour + 30*time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	tests := []string{
		"not-a-duration",
		"15",
		"abc",
		"-",
		"15 minutes",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			got := parseDuration(input)
			if got != defaultPollInterval {
				t.Errorf("parseDuration(%q) = %v, want default %v", input, got, defaultPollInterval)
			}
		})
	}
}

func TestParseDuration_Empty(t *testing.T) {
	got := parseDuration("")
	if got != defaultPollInterval {
		t.Errorf("parseDuration(\"\") = %v, want default %v", got, defaultPollInterval)
	}
}
