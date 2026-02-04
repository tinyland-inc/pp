package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// awsDashboardURL is the AWS billing console link.
	awsDashboardURL = "https://console.aws.amazon.com/billing/home"

	// awsCERegion is the region used for Cost Explorer API calls.
	// Cost Explorer is a global service but requires a region parameter.
	awsCERegion = "us-east-1"
)

// awsCLICommand is the path to the AWS CLI binary. It is a package-level
// variable so tests can override it with a mock script.
var awsCLICommand = "aws"

// awsCostResponse represents the JSON output of `aws ce get-cost-and-usage`.
type awsCostResponse struct {
	ResultsByTime []awsCostResult `json:"ResultsByTime"`
}

// awsCostResult is a single time-period entry in the Cost Explorer response.
type awsCostResult struct {
	TimePeriod awsTimePeriod        `json:"TimePeriod"`
	Total      map[string]awsMetric `json:"Total"`
}

// awsTimePeriod holds the start and end dates for a billing period.
type awsTimePeriod struct {
	Start string `json:"Start"`
	End   string `json:"End"`
}

// awsMetric holds a single metric value from Cost Explorer.
type awsMetric struct {
	Amount string `json:"Amount"`
	Unit   string `json:"Unit"`
}

// awsForecastResponse represents the JSON output of `aws ce get-cost-forecast`.
type awsForecastResponse struct {
	Total awsMetric `json:"Total"`
}

// AWSClient fetches billing data from AWS Cost Explorer by shelling out to the
// aws CLI. This avoids pulling in the AWS SDK as a dependency.
//
// AWS Cost Explorer charges $0.01 per API call. The billing collector's
// default 1-hour interval results in approximately $0.72/day ($0.01 * 3 calls
// * 24 hours). Consider increasing the interval to 6h for cost savings.
type AWSClient struct {
	profile string
	regions []string
	logger  *slog.Logger
}

// NewAWSClient creates an AWSClient that uses the given AWS CLI profile.
// If logger is nil, a no-op logger is used. The first entry in regions
// is used for CLI calls; if regions is empty, us-east-1 is used.
func NewAWSClient(profile string, regions []string, logger *slog.Logger) *AWSClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &AWSClient{
		profile: profile,
		regions: regions,
		logger:  logger,
	}
}

// FetchBilling retrieves the current month's billing data from AWS Cost Explorer
// via the aws CLI. It makes up to three CLI calls:
//
//  1. get-cost-and-usage for current month spend (month start to today)
//  2. get-cost-forecast for projected end-of-month spend (tomorrow to month end)
//  3. get-cost-and-usage for previous month's total spend
//
// If the aws CLI is not found, Status is set to "error". If authentication
// fails (non-zero exit), Status is set to "auth_failed". Forecast failures
// are non-fatal and result in a nil ForecastUSD.
func (c *AWSClient) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	now := time.Now().UTC()
	region := c.region()
	start, end := CurrentMonthRange()
	today := now.Format("2006-01-02")

	c.logger.Debug("fetching AWS billing",
		"profile", c.profile,
		"region", region,
		"period_start", start,
		"period_end", end,
	)

	// 1. Current month spend: from month start to today.
	currentSpend, err := c.fetchCurrentSpend(ctx, region, start, today)
	if err != nil {
		return c.errorResult(err, start, end, now), nil
	}

	// 2. Forecast: from tomorrow to month end. Non-fatal if it fails.
	var forecast *float64
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	// Only attempt forecast if tomorrow is before or equal to the month end.
	if tomorrow <= end {
		f, err := c.fetchForecast(ctx, region, tomorrow, end)
		if err != nil {
			c.logger.Warn("AWS forecast unavailable, skipping",
				"error", err,
				"profile", c.profile,
			)
		} else {
			forecast = f
		}
	}

	// 3. Previous month spend. Non-fatal if it fails.
	var prevMonth *float64
	prevStart, prevEnd := PreviousMonthRange()
	// The end date for get-cost-and-usage is exclusive, so use the first day
	// of the current month to capture the full previous month.
	prevEndExclusive := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	_ = prevEnd // we use prevEndExclusive for the API call
	prev, err := c.fetchSpend(ctx, region, prevStart, prevEndExclusive)
	if err != nil {
		c.logger.Warn("AWS previous month spend unavailable",
			"error", err,
			"profile", c.profile,
		)
	} else {
		v := RoundCents(prev)
		prevMonth = &v
	}

	return &collectors.ProviderBilling{
		Provider:     "aws",
		AccountName:  "AWS (" + c.profile + ")",
		Status:       "ok",
		DashboardURL: awsDashboardURL,
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    RoundCents(currentSpend),
			ForecastUSD: forecast,
			StartDate:   start,
			EndDate:     end,
		},
		PreviousMonth: prevMonth,
		FetchedAt:     now,
	}, nil
}

// region returns the AWS region to use for CLI calls. It prefers the first
// entry in c.regions, falling back to us-east-1.
func (c *AWSClient) region() string {
	if len(c.regions) > 0 && c.regions[0] != "" {
		return c.regions[0]
	}
	return awsCERegion
}

// fetchCurrentSpend retrieves the total unblended cost for the given period.
func (c *AWSClient) fetchCurrentSpend(ctx context.Context, region, start, end string) (float64, error) {
	return c.fetchSpend(ctx, region, start, end)
}

// fetchSpend runs `aws ce get-cost-and-usage` and sums the UnblendedCost
// across all returned time periods.
func (c *AWSClient) fetchSpend(ctx context.Context, region, start, end string) (float64, error) {
	args := []string{
		"ce", "get-cost-and-usage",
		"--profile", c.profile,
		"--region", region,
		"--time-period", fmt.Sprintf("Start=%s,End=%s", start, end),
		"--granularity", "MONTHLY",
		"--metrics", "UnblendedCost",
		"--output", "json",
	}

	out, err := c.runCLI(ctx, args)
	if err != nil {
		return 0, err
	}

	var resp awsCostResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return 0, fmt.Errorf("parsing AWS cost response: %w", err)
	}

	var total float64
	for _, result := range resp.ResultsByTime {
		metric, ok := result.Total["UnblendedCost"]
		if !ok {
			continue
		}
		amount, err := ParseAmount(metric.Amount)
		if err != nil {
			return 0, fmt.Errorf("parsing cost amount: %w", err)
		}
		total += amount
	}

	return total, nil
}

// fetchForecast runs `aws ce get-cost-forecast` and returns the forecasted
// total as a pointer. Returns nil on success with a zero forecast.
func (c *AWSClient) fetchForecast(ctx context.Context, region, start, end string) (*float64, error) {
	args := []string{
		"ce", "get-cost-forecast",
		"--profile", c.profile,
		"--region", region,
		"--time-period", fmt.Sprintf("Start=%s,End=%s", start, end),
		"--metric", "UNBLENDED_COST",
		"--granularity", "MONTHLY",
		"--output", "json",
	}

	out, err := c.runCLI(ctx, args)
	if err != nil {
		return nil, err
	}

	var resp awsForecastResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing AWS forecast response: %w", err)
	}

	amount, err := ParseAmount(resp.Total.Amount)
	if err != nil {
		return nil, fmt.Errorf("parsing forecast amount: %w", err)
	}

	rounded := RoundCents(amount)
	return &rounded, nil
}

// runCLI executes an aws CLI command and returns its stdout. It uses
// exec.CommandContext so the context deadline/cancellation is respected.
func (c *AWSClient) runCLI(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, awsCLICommand, args...)

	c.logger.Debug("running AWS CLI", "command", awsCLICommand, "args", args)

	out, err := cmd.Output()
	if err != nil {
		// Check if the binary was not found.
		if isExecNotFound(err) {
			return nil, fmt.Errorf("aws CLI not found: %w", err)
		}
		// Check for context cancellation.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Non-zero exit code typically means auth or permission error.
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			return nil, fmt.Errorf("aws CLI exited with code %d: %s", exitErr.ExitCode(), stderr)
		}
		return nil, fmt.Errorf("running aws CLI: %w", err)
	}

	return out, nil
}

// isExecNotFound returns true if the error indicates the binary was not found.
func isExecNotFound(err error) bool {
	var pathErr *exec.Error
	if ok := errorAs(err, &pathErr); ok {
		return pathErr.Err == exec.ErrNotFound
	}
	return false
}

// isExitError attempts to extract an *exec.ExitError from err.
func isExitError(err error, target **exec.ExitError) bool {
	return errorAs(err, target)
}

// errorAs is a thin wrapper around errors.As, separated for clarity.
// Using a function avoids importing "errors" when the standard library
// exec package already provides what we need.
func errorAs[T any](err error, target *T) bool {
	for err != nil {
		if t, ok := err.(T); ok {
			*target = t
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}

// errorResult builds a ProviderBilling with an error or auth_failed status
// based on the error type.
func (c *AWSClient) errorResult(err error, start, end string, now time.Time) *collectors.ProviderBilling {
	status := "error"
	if isExecNotFound(err) {
		status = "error"
	} else if !isExecNotFound(err) && strings.Contains(err.Error(), "aws CLI exited") {
		status = "auth_failed"
	}

	c.logger.Warn("AWS billing fetch failed",
		"error", err,
		"status", status,
		"profile", c.profile,
	)

	return &collectors.ProviderBilling{
		Provider:     "aws",
		AccountName:  "AWS (" + c.profile + ")",
		Status:       status,
		DashboardURL: awsDashboardURL,
		CurrentMonth: collectors.MonthCost{
			StartDate: start,
			EndDate:   end,
		},
		FetchedAt: now,
	}
}

// Compile-time interface compliance check.
var _ ProviderFetcher = (*AWSClient)(nil)
