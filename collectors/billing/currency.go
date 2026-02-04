package billing

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// RoundCents rounds a USD amount to 2 decimal places.
func RoundCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// ParseAmount safely parses a string amount (e.g., "23.44") to float64.
// Returns 0 and an error if parsing fails.
func ParseAmount(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing amount %q: %w", s, err)
	}
	return v, nil
}

// FormatUSD formats an amount as a USD string (e.g., "$23.44").
func FormatUSD(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// ForecastFromSpend calculates the projected end-of-month spend using a
// linear extrapolation of current spending.
//
// currentSpend: amount spent so far in USD.
// daysElapsed: number of days in the billing period so far (minimum 1).
// daysInMonth: total days in the billing month.
//
// If daysElapsed is less than 1, it defaults to 1 to avoid division by zero.
func ForecastFromSpend(currentSpend float64, daysElapsed, daysInMonth int) float64 {
	if daysElapsed < 1 {
		daysElapsed = 1
	}
	return RoundCents(currentSpend / float64(daysElapsed) * float64(daysInMonth))
}

// CurrentMonthRange returns the start (1st of month) and end (last day) dates
// for the current month as YYYY-MM-DD strings.
func CurrentMonthRange() (start, end string) {
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	return first.Format("2006-01-02"), last.Format("2006-01-02")
}

// PreviousMonthRange returns the start and end dates for the previous month
// as YYYY-MM-DD strings. Handles the January-to-December year boundary.
func PreviousMonthRange() (start, end string) {
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, time.UTC)
	return first.Format("2006-01-02"), last.Format("2006-01-02")
}

// DaysElapsedInMonth returns how many days have passed in the current month,
// including today. The minimum return value is 1.
func DaysElapsedInMonth() int {
	return time.Now().UTC().Day()
}

// DaysInMonth returns the total number of days in the given year/month.
func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
