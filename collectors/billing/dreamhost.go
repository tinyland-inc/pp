package billing

import (
	"context"
	"io"
	"log/slog"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// dreamhostDashboardURL is the DreamHost billing panel link.
	dreamhostDashboardURL = "https://panel.dreamhost.com/index.cgi?tree=billing.account"
)

// DreamHostClient provides limited billing data from the DreamHost API.
// DreamHost's API only supports bandwidth queries, not actual billing data.
// See research: docs/UNIFIED_CLOUD_BILLING_DASHBOARD_GUIDE.md
type DreamHostClient struct {
	apiKey string
	logger *slog.Logger
}

// NewDreamHostClient creates a DreamHostClient stub.
// If logger is nil, a no-op logger is used.
func NewDreamHostClient(apiKey string, logger *slog.Logger) *DreamHostClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &DreamHostClient{
		apiKey: apiKey,
		logger: logger,
	}
}

// FetchBilling returns a stub ProviderBilling noting DreamHost's limited API.
// The Status field is set to "limited" because DreamHost only exposes
// bandwidth data, not billing amounts.
func (c *DreamHostClient) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	c.logger.Debug("DreamHost billing stub called")

	start, end := CurrentMonthRange()

	return &collectors.ProviderBilling{
		Provider:     "dreamhost",
		AccountName:  "DreamHost (bandwidth only)",
		Status:       "limited",
		DashboardURL: dreamhostDashboardURL,
		CurrentMonth: collectors.MonthCost{
			StartDate: start,
			EndDate:   end,
		},
		FetchedAt: time.Now(),
	}, nil
}
