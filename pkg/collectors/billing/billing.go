package billing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Default configuration values.
const (
	DefaultInterval = 15 * time.Minute
)

// Config holds the configuration for the billing collector.
type Config struct {
	// Interval is how often collection runs. Zero uses DefaultInterval.
	Interval time.Duration

	// Civo holds API credentials for the Civo provider. Nil disables Civo.
	Civo *CivoConfig

	// DigitalOcean holds API credentials for DigitalOcean. Nil disables DO.
	DigitalOcean *DOConfig

	// BudgetUSD is the monthly budget for percentage calculation. Zero means
	// no budget is set, and BudgetPercent will be 0 in the report.
	BudgetUSD float64
}

// CivoConfig holds authentication details for the Civo API.
type CivoConfig struct {
	APIKey string
	Region string
}

// DOConfig holds authentication details for the DigitalOcean API.
type DOConfig struct {
	APIToken string
}

// BillingReport is the top-level data returned by Collect.
type BillingReport struct {
	Providers       []ProviderBilling `json:"providers"`
	TotalMonthlyUSD float64           `json:"total_monthly_usd"`
	BudgetUSD       float64           `json:"budget_usd"`
	BudgetPercent   float64           `json:"budget_percent"`
	Timestamp       time.Time         `json:"timestamp"`
}

// ProviderBilling contains billing data for a single cloud provider.
type ProviderBilling struct {
	Name        string         `json:"name"`
	Connected   bool           `json:"connected"`
	Error       string         `json:"error,omitempty"`
	MonthToDate float64        `json:"month_to_date"`
	Balance     float64        `json:"balance"`
	Resources   []ResourceCost `json:"resources"`
}

// ResourceCost represents the cost of a single cloud resource.
type ResourceCost struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	MonthlyCost float64 `json:"monthly_cost"`
	HourlyCost  float64 `json:"hourly_cost"`
}

// Collector gathers billing data from configured cloud providers.
type Collector struct {
	cfg      Config
	interval time.Duration

	civoClient CivoClient
	doClient   DOClient

	mu      sync.Mutex
	healthy bool
}

// New creates a new billing collector. If cfg.Interval is zero,
// DefaultInterval is used. Real HTTP clients are created for any
// non-nil provider config.
func New(cfg Config) *Collector {
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}

	c := &Collector{
		cfg:      cfg,
		interval: interval,
		healthy:  true,
	}

	if cfg.Civo != nil {
		c.civoClient = newCivoHTTPClient(cfg.Civo.APIKey, cfg.Civo.Region)
	}
	if cfg.DigitalOcean != nil {
		c.doClient = newDOHTTPClient(cfg.DigitalOcean.APIToken)
	}

	return c
}

// newWithClients creates a Collector with injected clients for testing.
func newWithClients(cfg Config, civo CivoClient, do DOClient) *Collector {
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Collector{
		cfg:        cfg,
		interval:   interval,
		civoClient: civo,
		doClient:   do,
		healthy:    true,
	}
}

// Name returns the collector identifier.
func (c *Collector) Name() string {
	return "billing"
}

// Interval returns how often this collector should run.
func (c *Collector) Interval() time.Duration {
	return c.interval
}

// Healthy returns whether the last collection succeeded.
func (c *Collector) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

// setHealthy updates the internal healthy flag under the mutex.
func (c *Collector) setHealthy(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = v
}

// Collect queries all configured providers concurrently and returns a
// BillingReport. Individual provider failures are captured in the report
// rather than failing the entire collection. The collector is marked
// unhealthy only if ALL configured providers fail.
func (c *Collector) Collect(ctx context.Context) (interface{}, error) {
	if err := ctx.Err(); err != nil {
		c.setHealthy(false)
		return nil, fmt.Errorf("billing collect: %w", err)
	}

	type providerResult struct {
		billing ProviderBilling
	}

	var wg sync.WaitGroup
	var civoResult, doResult *providerResult

	// Query Civo concurrently if configured.
	if c.civoClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pb := c.collectCivo(ctx)
			civoResult = &providerResult{billing: pb}
		}()
	}

	// Query DigitalOcean concurrently if configured.
	if c.doClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pb := c.collectDO(ctx)
			doResult = &providerResult{billing: pb}
		}()
	}

	wg.Wait()

	report := &BillingReport{
		BudgetUSD: c.cfg.BudgetUSD,
		Timestamp: time.Now(),
	}

	configuredCount := 0
	failedCount := 0

	if civoResult != nil {
		configuredCount++
		report.Providers = append(report.Providers, civoResult.billing)
		if civoResult.billing.Connected {
			report.TotalMonthlyUSD += civoResult.billing.MonthToDate
		} else {
			failedCount++
		}
	}

	if doResult != nil {
		configuredCount++
		report.Providers = append(report.Providers, doResult.billing)
		if doResult.billing.Connected {
			report.TotalMonthlyUSD += doResult.billing.MonthToDate
		} else {
			failedCount++
		}
	}

	// Ensure Providers is never nil for consistent JSON serialization.
	if report.Providers == nil {
		report.Providers = []ProviderBilling{}
	}

	// Calculate budget percentage.
	if c.cfg.BudgetUSD > 0 {
		report.BudgetPercent = (report.TotalMonthlyUSD / c.cfg.BudgetUSD) * 100
	}

	// Mark unhealthy only if all configured providers failed.
	if configuredCount > 0 && failedCount == configuredCount {
		c.setHealthy(false)
	} else {
		c.setHealthy(true)
	}

	return report, nil
}

// collectCivo queries the Civo API and returns a ProviderBilling result.
func (c *Collector) collectCivo(ctx context.Context) ProviderBilling {
	pb := ProviderBilling{
		Name:      "civo",
		Resources: []ResourceCost{},
	}

	// Fetch Kubernetes clusters.
	k8s, err := c.civoClient.GetKubernetes(ctx)
	if err != nil {
		pb.Error = err.Error()
		return pb
	}

	if k8s != nil {
		for _, cluster := range k8s.Items {
			pb.Resources = append(pb.Resources, ResourceCost{
				Name:        cluster.Name,
				Type:        "kubernetes",
				MonthlyCost: cluster.MonthlyCost,
			})
			pb.MonthToDate += cluster.MonthlyCost
		}
	}

	// Fetch compute instances.
	instances, err := c.civoClient.GetInstances(ctx)
	if err != nil {
		pb.Error = err.Error()
		return pb
	}

	if instances != nil {
		for _, inst := range instances.Items {
			pb.Resources = append(pb.Resources, ResourceCost{
				Name:        inst.Hostname,
				Type:        "instance",
				MonthlyCost: inst.MonthlyCost,
			})
			pb.MonthToDate += inst.MonthlyCost
		}
	}

	pb.Connected = true
	return pb
}

// collectDO queries the DigitalOcean API and returns a ProviderBilling result.
func (c *Collector) collectDO(ctx context.Context) ProviderBilling {
	pb := ProviderBilling{
		Name:      "digitalocean",
		Resources: []ResourceCost{},
	}

	// Fetch account balance (month-to-date and credits).
	balance, err := c.doClient.GetBalance(ctx)
	if err != nil {
		pb.Error = err.Error()
		return pb
	}

	if balance != nil {
		mtd, err := balance.ParseMonthToDate()
		if err != nil {
			pb.Error = fmt.Sprintf("parsing month-to-date balance: %v", err)
			return pb
		}
		pb.MonthToDate = mtd

		acctBal, err := balance.ParseAccountBalance()
		if err != nil {
			pb.Error = fmt.Sprintf("parsing account balance: %v", err)
			return pb
		}
		pb.Balance = acctBal
	}

	// Fetch DOKS clusters.
	k8s, err := c.doClient.GetKubernetes(ctx)
	if err != nil {
		pb.Error = err.Error()
		return pb
	}

	if k8s != nil {
		for _, cluster := range k8s.KubernetesClusters {
			// Sum node pool costs by looking up size pricing from droplets.
			// For now, we record the cluster as a single resource without
			// per-node pricing (would require additional size lookup).
			pb.Resources = append(pb.Resources, ResourceCost{
				Name: cluster.Name,
				Type: "kubernetes",
			})
		}
	}

	// Fetch droplets.
	droplets, err := c.doClient.GetDroplets(ctx)
	if err != nil {
		pb.Error = err.Error()
		return pb
	}

	if droplets != nil {
		for _, d := range droplets.Droplets {
			pb.Resources = append(pb.Resources, ResourceCost{
				Name:        d.Name,
				Type:        "droplet",
				MonthlyCost: d.Size.PriceMonthly,
				HourlyCost:  d.Size.PriceHourly,
			})
		}
	}

	pb.Connected = true
	return pb
}
