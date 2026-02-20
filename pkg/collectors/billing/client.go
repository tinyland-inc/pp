// Package billing provides a collector that aggregates cloud billing data from
// Civo and DigitalOcean APIs. Each provider is queried independently; failures
// in one provider do not prevent collection from the other.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Civo API types and client
// ---------------------------------------------------------------------------

// CivoClient abstracts the Civo API for testability.
type CivoClient interface {
	GetCharges(ctx context.Context) (*CivoChargesResponse, error)
	GetKubernetes(ctx context.Context) (*CivoK8sResponse, error)
	GetInstances(ctx context.Context) (*CivoInstancesResponse, error)
	GetSizes(ctx context.Context) (*CivoSizesResponse, error)
}

// CivoChargesResponse represents the response from GET /v2/charges.
type CivoChargesResponse struct {
	Items []CivoCharge `json:"items"`
}

// CivoCharge is a single charge line item.
type CivoCharge struct {
	Code      string  `json:"code"`
	Label     string  `json:"label"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	NumHours  int     `json:"num_hours"`
	SizeGB    float64 `json:"size_gb"`
	TotalCost float64 `json:"total_cost"`
}

// CivoK8sResponse represents the response from GET /v2/kubernetes.
type CivoK8sResponse struct {
	Items []CivoK8sCluster `json:"items"`
}

// CivoK8sCluster is a Kubernetes cluster from the Civo API.
type CivoK8sCluster struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	MonthlyCost     float64 `json:"monthly_cost"`
	NumTargetNodes  int     `json:"num_target_nodes"`
	TargetNodesSize string  `json:"target_nodes_size"`
}

// CivoSizesResponse represents the response from GET /v2/sizes.
type CivoSizesResponse struct {
	Items []CivoSize `json:"items"`
}

// CivoSize represents an instance size with pricing information.
type CivoSize struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	CPUCores     int     `json:"cpu_cores"`
	RAMMb        int     `json:"ram_mb"`
	DiskGb       int     `json:"disk_gb"`
	PriceMonthly float64 `json:"price_monthly"`
	PriceHourly  float64 `json:"price_hourly"`
}

// CivoInstancesResponse represents the response from GET /v2/instances.
type CivoInstancesResponse struct {
	Items []CivoInstance `json:"items"`
}

// CivoInstance is a compute instance from the Civo API.
type CivoInstance struct {
	ID          string  `json:"id"`
	Hostname    string  `json:"hostname"`
	Status      string  `json:"status"`
	Size        string  `json:"size"`
	MonthlyCost float64 `json:"monthly_cost"`
}

// civoHTTPClient implements CivoClient using net/http.
type civoHTTPClient struct {
	baseURL string
	apiKey  string
	region  string
	client  *http.Client
}

func newCivoHTTPClient(apiKey, region string) *civoHTTPClient {
	return &civoHTTPClient{
		baseURL: "https://api.civo.com/v2",
		apiKey:  apiKey,
		region:  region,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *civoHTTPClient) doRequest(ctx context.Context, path string, out interface{}) error {
	url := c.baseURL + path
	if c.region != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		url += sep + "region=" + c.region
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("civo API %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func (c *civoHTTPClient) GetCharges(ctx context.Context) (*CivoChargesResponse, error) {
	// CIVO /v2/charges returns a raw JSON array, not {items: [...]}.
	var charges []CivoCharge
	if err := c.doRequest(ctx, "/charges", &charges); err != nil {
		return nil, err
	}
	return &CivoChargesResponse{Items: charges}, nil
}

func (c *civoHTTPClient) GetKubernetes(ctx context.Context) (*CivoK8sResponse, error) {
	var resp CivoK8sResponse
	if err := c.doRequest(ctx, "/kubernetes/clusters", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *civoHTTPClient) GetInstances(ctx context.Context) (*CivoInstancesResponse, error) {
	var resp CivoInstancesResponse
	if err := c.doRequest(ctx, "/instances", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *civoHTTPClient) GetSizes(ctx context.Context) (*CivoSizesResponse, error) {
	var resp CivoSizesResponse
	if err := c.doRequest(ctx, "/sizes", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// DigitalOcean API types and client
// ---------------------------------------------------------------------------

// DOClient abstracts the DigitalOcean API for testability.
type DOClient interface {
	GetBalance(ctx context.Context) (*DOBalanceResponse, error)
	GetKubernetes(ctx context.Context) (*DOK8sResponse, error)
	GetDroplets(ctx context.Context) (*DODropletsResponse, error)
}

// DOBalanceResponse represents the response from GET /v2/customers/balance.
// DigitalOcean returns monetary amounts as strings.
type DOBalanceResponse struct {
	MonthToDateBalance string `json:"month_to_date_balance"`
	AccountBalance     string `json:"account_balance"`
	MonthToDateUsage   string `json:"month_to_date_usage"`
	GeneratedAt        string `json:"generated_at"`
}

// ParseMonthToDate parses the month-to-date balance as a float64.
func (r *DOBalanceResponse) ParseMonthToDate() (float64, error) {
	if r == nil || r.MonthToDateBalance == "" {
		return 0, nil
	}
	return strconv.ParseFloat(r.MonthToDateBalance, 64)
}

// ParseAccountBalance parses the account balance as a float64.
func (r *DOBalanceResponse) ParseAccountBalance() (float64, error) {
	if r == nil || r.AccountBalance == "" {
		return 0, nil
	}
	return strconv.ParseFloat(r.AccountBalance, 64)
}

// DOK8sResponse represents the response from GET /v2/kubernetes/clusters.
type DOK8sResponse struct {
	KubernetesClusters []DOK8sCluster `json:"kubernetes_clusters"`
}

// DOK8sCluster is a DOKS cluster.
type DOK8sCluster struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Status    DOK8sStatus  `json:"status"`
	NodePools []DONodePool `json:"node_pools"`
}

// DOK8sStatus represents cluster status.
type DOK8sStatus struct {
	State string `json:"state"`
}

// DONodePool is a node pool within a DOKS cluster.
type DONodePool struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Size  string `json:"size"`
	Count int    `json:"count"`
	Nodes []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"nodes"`
}

// DODropletsResponse represents the response from GET /v2/droplets.
type DODropletsResponse struct {
	Droplets []DODroplet `json:"droplets"`
}

// DODroplet is a single droplet from the DigitalOcean API.
type DODroplet struct {
	ID   int       `json:"id"`
	Name string    `json:"name"`
	Size DOSize    `json:"size"`
}

// DOSize contains the pricing information for a droplet or node pool.
type DOSize struct {
	Slug         string  `json:"slug"`
	PriceMonthly float64 `json:"price_monthly"`
	PriceHourly  float64 `json:"price_hourly"`
}

// doHTTPClient implements DOClient using net/http.
type doHTTPClient struct {
	baseURL  string
	apiToken string
	client   *http.Client
}

func newDOHTTPClient(apiToken string) *doHTTPClient {
	return &doHTTPClient{
		baseURL:  "https://api.digitalocean.com/v2",
		apiToken: apiToken,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *doHTTPClient) doRequest(ctx context.Context, path string, out interface{}) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("digitalocean API %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func (c *doHTTPClient) GetBalance(ctx context.Context) (*DOBalanceResponse, error) {
	var resp DOBalanceResponse
	if err := c.doRequest(ctx, "/customers/my/balance", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *doHTTPClient) GetKubernetes(ctx context.Context) (*DOK8sResponse, error) {
	var resp DOK8sResponse
	if err := c.doRequest(ctx, "/kubernetes/clusters", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *doHTTPClient) GetDroplets(ctx context.Context) (*DODropletsResponse, error) {
	var resp DODropletsResponse
	if err := c.doRequest(ctx, "/droplets", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
