package billing

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeScript creates an executable script in a temp directory that writes
// the given stdout to os.Stdout and exits with the given code. It returns
// the path to the script.
func writeScript(t *testing.T, stdout string, exitCode int) string {
	t.Helper()

	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		// Windows batch file (unlikely for this project, but handle gracefully).
		script := filepath.Join(dir, "aws.bat")
		content := "@echo off\n"
		if stdout != "" {
			content += "echo " + stdout + "\n"
		}
		content += "exit /b " + itoa(exitCode) + "\n"
		if err := os.WriteFile(script, []byte(content), 0755); err != nil {
			t.Fatalf("writing script: %v", err)
		}
		return script
	}

	// Unix shell script.
	script := filepath.Join(dir, "aws")
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	if stdout != "" {
		// Use printf to avoid trailing newline issues with echo.
		sb.WriteString("printf '%s' '")
		sb.WriteString(stdout)
		sb.WriteString("'\n")
	}
	if exitCode != 0 {
		// Write an error message to stderr for exit error tests.
		sb.WriteString("echo 'AccessDeniedException: User is not authorized' >&2\n")
	}
	sb.WriteString("exit ")
	sb.WriteString(itoa(exitCode))
	sb.WriteString("\n")

	if err := os.WriteFile(script, []byte(sb.String()), 0755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	return script
}

// itoa converts an int to a string without importing strconv in tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// withAWSCLI overrides the package-level awsCLICommand variable for the
// duration of fn, restoring it afterward.
func withAWSCLI(path string, fn func()) {
	orig := awsCLICommand
	awsCLICommand = path
	defer func() { awsCLICommand = orig }()
	fn()
}

// --- Canned JSON responses ---

const cannedCostResponse = `{
  "ResultsByTime": [
    {
      "TimePeriod": {
        "Start": "2026-02-01",
        "End": "2026-02-03"
      },
      "Total": {
        "UnblendedCost": {
          "Amount": "42.37",
          "Unit": "USD"
        }
      }
    }
  ]
}`

const cannedForecastResponse = `{
  "Total": {
    "Amount": "156.80",
    "Unit": "USD"
  }
}`

const cannedPrevMonthResponse = `{
  "ResultsByTime": [
    {
      "TimePeriod": {
        "Start": "2026-01-01",
        "End": "2026-02-01"
      },
      "Total": {
        "UnblendedCost": {
          "Amount": "134.56",
          "Unit": "USD"
        }
      }
    }
  ]
}`

const cannedZeroSpendResponse = `{
  "ResultsByTime": [
    {
      "TimePeriod": {
        "Start": "2026-02-01",
        "End": "2026-02-03"
      },
      "Total": {
        "UnblendedCost": {
          "Amount": "0.0",
          "Unit": "USD"
        }
      }
    }
  ]
}`

// writeMultiCallScript creates a script that returns different JSON depending
// on which aws subcommand is called. It inspects $2 (the second arg after "ce")
// to decide.
func writeMultiCallScript(t *testing.T, costJSON, forecastJSON, prevJSON string) string {
	t.Helper()

	dir := t.TempDir()
	script := filepath.Join(dir, "aws")

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Route based on the second argument (get-cost-and-usage vs get-cost-forecast)\n")
	sb.WriteString("subcmd=\"$2\"\n")
	sb.WriteString("case \"$subcmd\" in\n")

	sb.WriteString("  get-cost-forecast)\n")
	if forecastJSON == "" {
		sb.WriteString("    echo 'DataUnavailableException: insufficient data' >&2\n")
		sb.WriteString("    exit 1\n")
	} else {
		sb.WriteString("    printf '%s' '")
		sb.WriteString(forecastJSON)
		sb.WriteString("'\n")
		sb.WriteString("    exit 0\n")
	}
	sb.WriteString("    ;;\n")

	sb.WriteString("  get-cost-and-usage)\n")
	// Distinguish current vs previous month by checking the --time-period arg.
	// Previous month calls will have a Start date from the previous month.
	sb.WriteString("    # Check if this is a previous month query by looking at all args\n")
	sb.WriteString("    for arg in \"$@\"; do\n")
	sb.WriteString("      case \"$arg\" in\n")
	sb.WriteString("        Start=????-??-01,End=????-??-01)\n")
	// This pattern matches previous month queries where end is first of current month.
	if prevJSON != "" {
		sb.WriteString("          printf '%s' '")
		sb.WriteString(prevJSON)
		sb.WriteString("'\n")
		sb.WriteString("          exit 0\n")
	} else {
		sb.WriteString("          echo 'error fetching previous month' >&2\n")
		sb.WriteString("          exit 1\n")
	}
	sb.WriteString("          ;;\n")
	sb.WriteString("      esac\n")
	sb.WriteString("    done\n")
	// Default: current month query.
	sb.WriteString("    printf '%s' '")
	sb.WriteString(costJSON)
	sb.WriteString("'\n")
	sb.WriteString("    exit 0\n")
	sb.WriteString("    ;;\n")

	sb.WriteString("  *)\n")
	sb.WriteString("    echo 'unknown subcommand' >&2\n")
	sb.WriteString("    exit 1\n")
	sb.WriteString("    ;;\n")
	sb.WriteString("esac\n")

	if err := os.WriteFile(script, []byte(sb.String()), 0755); err != nil {
		t.Fatalf("writing multi-call script: %v", err)
	}

	return script
}

// --- Tests ---

func TestAWSClient_NewAWSClient(t *testing.T) {
	client := NewAWSClient("default", []string{"us-east-1"}, nil)
	if client == nil {
		t.Fatal("NewAWSClient returned nil")
	}
	if client.profile != "default" {
		t.Errorf("profile = %q, want %q", client.profile, "default")
	}
	if len(client.regions) != 1 || client.regions[0] != "us-east-1" {
		t.Errorf("regions = %v, want [us-east-1]", client.regions)
	}
}

func TestAWSClient_NilLogger(t *testing.T) {
	// Ensure NewAWSClient does not panic with nil logger.
	client := NewAWSClient("test", []string{"eu-west-1"}, nil)
	if client == nil {
		t.Fatal("NewAWSClient returned nil")
	}
}

func TestAWSClient_Region_Default(t *testing.T) {
	client := NewAWSClient("default", nil, nil)
	if got := client.region(); got != awsCERegion {
		t.Errorf("region() = %q, want %q", got, awsCERegion)
	}
}

func TestAWSClient_Region_FromConfig(t *testing.T) {
	client := NewAWSClient("default", []string{"eu-west-1", "us-west-2"}, nil)
	if got := client.region(); got != "eu-west-1" {
		t.Errorf("region() = %q, want %q", got, "eu-west-1")
	}
}

func TestAWSClient_Region_EmptySlice(t *testing.T) {
	client := NewAWSClient("default", []string{}, nil)
	if got := client.region(); got != awsCERegion {
		t.Errorf("region() = %q, want %q", got, awsCERegion)
	}
}

func TestAWSClient_FetchBilling_Success(t *testing.T) {
	script := writeMultiCallScript(t, cannedCostResponse, cannedForecastResponse, cannedPrevMonthResponse)

	client := NewAWSClient("prod", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}
		if result == nil {
			t.Fatal("FetchBilling() returned nil")
		}

		if result.Provider != "aws" {
			t.Errorf("Provider = %q, want %q", result.Provider, "aws")
		}
		if result.Status != "ok" {
			t.Errorf("Status = %q, want %q", result.Status, "ok")
		}
		if result.AccountName != "AWS (prod)" {
			t.Errorf("AccountName = %q, want %q", result.AccountName, "AWS (prod)")
		}
		if result.DashboardURL != awsDashboardURL {
			t.Errorf("DashboardURL = %q, want %q", result.DashboardURL, awsDashboardURL)
		}

		// Current month spend: 42.37.
		if result.CurrentMonth.SpendUSD != 42.37 {
			t.Errorf("SpendUSD = %v, want 42.37", result.CurrentMonth.SpendUSD)
		}

		// Forecast should be present.
		if result.CurrentMonth.ForecastUSD == nil {
			t.Fatal("ForecastUSD is nil, want non-nil")
		}
		if *result.CurrentMonth.ForecastUSD != 156.80 {
			t.Errorf("ForecastUSD = %v, want 156.80", *result.CurrentMonth.ForecastUSD)
		}

		// Previous month should be present.
		if result.PreviousMonth == nil {
			t.Fatal("PreviousMonth is nil, want non-nil")
		}
		if *result.PreviousMonth != 134.56 {
			t.Errorf("PreviousMonth = %v, want 134.56", *result.PreviousMonth)
		}

		// Dates should be set.
		if result.CurrentMonth.StartDate == "" {
			t.Error("StartDate is empty")
		}
		if result.CurrentMonth.EndDate == "" {
			t.Error("EndDate is empty")
		}

		// FetchedAt should be recent.
		if result.FetchedAt.IsZero() {
			t.Error("FetchedAt is zero")
		}
	})
}

func TestAWSClient_FetchBilling_ForecastUnavailable(t *testing.T) {
	// Forecast returns exit code 1 (DataUnavailableException).
	script := writeMultiCallScript(t, cannedCostResponse, "", cannedPrevMonthResponse)

	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}

		if result.Status != "ok" {
			t.Errorf("Status = %q, want %q", result.Status, "ok")
		}

		// Spend should still be present.
		if result.CurrentMonth.SpendUSD != 42.37 {
			t.Errorf("SpendUSD = %v, want 42.37", result.CurrentMonth.SpendUSD)
		}

		// Forecast should be nil (graceful degradation).
		if result.CurrentMonth.ForecastUSD != nil {
			t.Errorf("ForecastUSD = %v, want nil", *result.CurrentMonth.ForecastUSD)
		}

		// Previous month should still work.
		if result.PreviousMonth == nil {
			t.Fatal("PreviousMonth is nil, want non-nil")
		}
	})
}

func TestAWSClient_FetchBilling_CLINotFound(t *testing.T) {
	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	withAWSCLI("/nonexistent/path/aws-does-not-exist", func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() should not return error, got: %v", err)
		}

		if result.Status != "error" {
			t.Errorf("Status = %q, want %q", result.Status, "error")
		}
		if result.Provider != "aws" {
			t.Errorf("Provider = %q, want %q", result.Provider, "aws")
		}
	})
}

func TestAWSClient_FetchBilling_AuthFailure(t *testing.T) {
	// Script exits with code 1 and error on stderr.
	script := writeScript(t, "", 1)

	client := NewAWSClient("bad-profile", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() should not return error, got: %v", err)
		}

		if result.Status != "auth_failed" {
			t.Errorf("Status = %q, want %q", result.Status, "auth_failed")
		}
		if result.AccountName != "AWS (bad-profile)" {
			t.Errorf("AccountName = %q, want %q", result.AccountName, "AWS (bad-profile)")
		}
	})
}

func TestAWSClient_FetchBilling_ContextCancellation(t *testing.T) {
	// Create a script that blocks long enough for the context to expire.
	// Using exec with sleep directly so the process can be killed cleanly.
	dir := t.TempDir()
	script := filepath.Join(dir, "aws")
	content := "#!/bin/sh\nexec sleep 60\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(ctx)
		if err != nil {
			t.Fatalf("FetchBilling() should not return error, got: %v", err)
		}

		// The result should indicate an error since the CLI was killed.
		if result.Status == "ok" {
			t.Error("Status should not be ok when context is cancelled")
		}
	})
}

func TestAWSClient_FetchBilling_ZeroSpend(t *testing.T) {
	zeroForecast := `{"Total": {"Amount": "0.0", "Unit": "USD"}}`
	zeroPrev := `{
  "ResultsByTime": [
    {
      "TimePeriod": {"Start": "2026-01-01", "End": "2026-02-01"},
      "Total": {"UnblendedCost": {"Amount": "0.0", "Unit": "USD"}}
    }
  ]
}`

	script := writeMultiCallScript(t, cannedZeroSpendResponse, zeroForecast, zeroPrev)

	client := NewAWSClient("free-tier", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}

		if result.Status != "ok" {
			t.Errorf("Status = %q, want %q", result.Status, "ok")
		}
		if result.CurrentMonth.SpendUSD != 0.0 {
			t.Errorf("SpendUSD = %v, want 0.0", result.CurrentMonth.SpendUSD)
		}
		if result.CurrentMonth.ForecastUSD == nil {
			t.Fatal("ForecastUSD is nil, want non-nil (even if zero)")
		}
		if *result.CurrentMonth.ForecastUSD != 0.0 {
			t.Errorf("ForecastUSD = %v, want 0.0", *result.CurrentMonth.ForecastUSD)
		}
		if result.PreviousMonth == nil {
			t.Fatal("PreviousMonth is nil, want non-nil (even if zero)")
		}
		if *result.PreviousMonth != 0.0 {
			t.Errorf("PreviousMonth = %v, want 0.0", *result.PreviousMonth)
		}
	})
}

func TestAWSClient_FetchBilling_MalformedJSON(t *testing.T) {
	// Script returns invalid JSON.
	script := writeScript(t, "this is not json", 0)

	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() should not return error, got: %v", err)
		}

		// Malformed JSON in the cost call should result in an error status.
		if result.Status == "ok" {
			t.Error("Status should not be ok for malformed JSON response")
		}
	})
}

func TestAWSClient_FetchBilling_EmptyResultsByTime(t *testing.T) {
	emptyResults := `{"ResultsByTime": []}`
	script := writeMultiCallScript(t, emptyResults, cannedForecastResponse, emptyResults)

	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}

		if result.Status != "ok" {
			t.Errorf("Status = %q, want %q", result.Status, "ok")
		}
		// Empty results should give zero spend.
		if result.CurrentMonth.SpendUSD != 0.0 {
			t.Errorf("SpendUSD = %v, want 0.0", result.CurrentMonth.SpendUSD)
		}
	})
}

func TestAWSClient_FetchBilling_MultipleTimePeriods(t *testing.T) {
	multiPeriod := `{
  "ResultsByTime": [
    {
      "TimePeriod": {"Start": "2026-02-01", "End": "2026-02-15"},
      "Total": {"UnblendedCost": {"Amount": "20.00", "Unit": "USD"}}
    },
    {
      "TimePeriod": {"Start": "2026-02-15", "End": "2026-02-28"},
      "Total": {"UnblendedCost": {"Amount": "30.00", "Unit": "USD"}}
    }
  ]
}`

	script := writeMultiCallScript(t, multiPeriod, cannedForecastResponse, cannedPrevMonthResponse)

	client := NewAWSClient("default", []string{"us-east-1"}, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}

		// Should sum: 20.00 + 30.00 = 50.00.
		if result.CurrentMonth.SpendUSD != 50.00 {
			t.Errorf("SpendUSD = %v, want 50.00", result.CurrentMonth.SpendUSD)
		}
	})
}

func TestAWSClient_DashboardURL(t *testing.T) {
	script := writeMultiCallScript(t, cannedCostResponse, cannedForecastResponse, cannedPrevMonthResponse)

	client := NewAWSClient("test", nil, nil)

	withAWSCLI(script, func() {
		result, err := client.FetchBilling(context.Background())
		if err != nil {
			t.Fatalf("FetchBilling() returned error: %v", err)
		}

		if !strings.Contains(result.DashboardURL, "aws.amazon.com") {
			t.Errorf("DashboardURL = %q, want URL containing %q",
				result.DashboardURL, "aws.amazon.com")
		}
	})
}

func TestAWSClient_InterfaceCompliance(t *testing.T) {
	// Verify AWSClient implements ProviderFetcher at compile time.
	var _ ProviderFetcher = (*AWSClient)(nil)
}
