package billing

import (
	"strings"
	"testing"
	"time"
)

func TestRoundCents(t *testing.T) {
	tests := []struct {
		name   string
		input  float64
		expect float64
	}{
		{"zero", 0, 0},
		// Note: 1.005 in IEEE 754 is actually 1.00499..., so math.Round gives 1.00.
		// This is expected behavior for float64 rounding.
		{"rounding_at_boundary", 1.005, 1.0},
		{"rounding_up", 1.006, 1.01},
		{"truncate_down", 23.449, 23.45},
		{"round_half_up", 23.455, 23.46},
		{"exact_cents", 42.10, 42.10},
		{"negative", -5.555, -5.56},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RoundCents(tc.input)
			if got != tc.expect {
				t.Errorf("RoundCents(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

func TestParseAmount_Valid(t *testing.T) {
	tests := []struct {
		input  string
		expect float64
	}{
		{"23.44", 23.44},
		{"0", 0},
		{"1234.56", 1234.56},
		{"-10.5", -10.5},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseAmount(tc.input)
			if err != nil {
				t.Fatalf("ParseAmount(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.expect {
				t.Errorf("ParseAmount(%q) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

func TestParseAmount_Invalid(t *testing.T) {
	invalid := []string{"", "abc", "12.34.56", "$100"}

	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			got, err := ParseAmount(input)
			if err == nil {
				t.Errorf("ParseAmount(%q) = %v, expected error", input, got)
			}
			if got != 0 {
				t.Errorf("ParseAmount(%q) = %v on error, want 0", input, got)
			}
		})
	}
}

func TestFormatUSD(t *testing.T) {
	tests := []struct {
		name   string
		input  float64
		expect string
	}{
		{"zero", 0, "$0.00"},
		{"cents", 23.44, "$23.44"},
		{"large", 1234.56, "$1234.56"},
		{"whole", 100, "$100.00"},
		{"negative", -5.50, "$-5.50"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatUSD(tc.input)
			if got != tc.expect {
				t.Errorf("FormatUSD(%v) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestForecastFromSpend(t *testing.T) {
	tests := []struct {
		name         string
		currentSpend float64
		daysElapsed  int
		daysInMonth  int
		expect       float64
	}{
		{"normal_mid_month", 50.0, 15, 30, 100.0},
		{"day_one", 10.0, 1, 31, 310.0},
		{"last_day", 300.0, 31, 31, 300.0},
		{"early_month", 5.0, 3, 30, 50.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ForecastFromSpend(tc.currentSpend, tc.daysElapsed, tc.daysInMonth)
			if got != tc.expect {
				t.Errorf("ForecastFromSpend(%v, %d, %d) = %v, want %v",
					tc.currentSpend, tc.daysElapsed, tc.daysInMonth, got, tc.expect)
			}
		})
	}
}

func TestForecastFromSpend_ZeroDays(t *testing.T) {
	// daysElapsed=0 should default to 1 to avoid division by zero.
	got := ForecastFromSpend(10.0, 0, 30)
	expect := 300.0
	if got != expect {
		t.Errorf("ForecastFromSpend(10.0, 0, 30) = %v, want %v", got, expect)
	}

	// Negative days should also default to 1.
	got = ForecastFromSpend(10.0, -5, 30)
	if got != expect {
		t.Errorf("ForecastFromSpend(10.0, -5, 30) = %v, want %v", got, expect)
	}
}

func TestCurrentMonthRange(t *testing.T) {
	start, end := CurrentMonthRange()

	// Verify YYYY-MM-DD format.
	if _, err := time.Parse("2006-01-02", start); err != nil {
		t.Errorf("start date %q is not YYYY-MM-DD: %v", start, err)
	}
	if _, err := time.Parse("2006-01-02", end); err != nil {
		t.Errorf("end date %q is not YYYY-MM-DD: %v", end, err)
	}

	// Start should be the 1st.
	if !strings.HasSuffix(start, "-01") {
		t.Errorf("start date %q should end with -01", start)
	}

	// Start and end should share the same year-month prefix.
	if start[:7] != end[:7] {
		t.Errorf("start %q and end %q should be in the same month", start, end)
	}
}

func TestPreviousMonthRange(t *testing.T) {
	start, end := PreviousMonthRange()

	startDate, err := time.Parse("2006-01-02", start)
	if err != nil {
		t.Fatalf("start date %q is not YYYY-MM-DD: %v", start, err)
	}
	endDate, err := time.Parse("2006-01-02", end)
	if err != nil {
		t.Fatalf("end date %q is not YYYY-MM-DD: %v", end, err)
	}

	// Start should be the 1st.
	if startDate.Day() != 1 {
		t.Errorf("start date day = %d, want 1", startDate.Day())
	}

	// Should be the previous month relative to now.
	now := time.Now().UTC()
	expectedMonth := now.Month() - 1
	expectedYear := now.Year()
	if expectedMonth == 0 {
		expectedMonth = time.December
		expectedYear--
	}

	if startDate.Month() != expectedMonth || startDate.Year() != expectedYear {
		t.Errorf("start date month/year = %v/%d, want %v/%d",
			startDate.Month(), startDate.Year(), expectedMonth, expectedYear)
	}

	// End should be the last day of the previous month.
	expectedEnd := DaysInMonth(expectedYear, expectedMonth)
	if endDate.Day() != expectedEnd {
		t.Errorf("end date day = %d, want %d", endDate.Day(), expectedEnd)
	}
}

func TestDaysElapsedInMonth(t *testing.T) {
	days := DaysElapsedInMonth()
	if days < 1 {
		t.Errorf("DaysElapsedInMonth() = %d, want >= 1", days)
	}
	if days > 31 {
		t.Errorf("DaysElapsedInMonth() = %d, want <= 31", days)
	}
}

func TestDaysInMonth_Currency(t *testing.T) {
	tests := []struct {
		name   string
		year   int
		month  time.Month
		expect int
	}{
		{"jan", 2025, time.January, 31},
		{"feb_non_leap", 2025, time.February, 28},
		{"feb_leap", 2024, time.February, 29},
		{"apr", 2025, time.April, 30},
		{"dec", 2025, time.December, 31},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DaysInMonth(tc.year, tc.month)
			if got != tc.expect {
				t.Errorf("DaysInMonth(%d, %v) = %d, want %d",
					tc.year, tc.month, got, tc.expect)
			}
		})
	}
}
