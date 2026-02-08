// Package infra provides infrastructure status collectors for prompt-pulse.
// It gathers mesh network and cluster state from Tailscale and Kubernetes
// APIs, returning canonical data structures for dashboard rendering.
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// tsAPIBaseURL is the Tailscale API base URL.
	tsAPIBaseURL = "https://api.tailscale.com/api/v2"

	// tsRequestTimeout is the per-client HTTP timeout.
	tsRequestTimeout = 10 * time.Second

	// tsUserAgent identifies prompt-pulse in request headers.
	tsUserAgent = "prompt-pulse/0.1.0"

	// tsMaxResponseBytes caps response body reads to prevent unbounded memory use.
	tsMaxResponseBytes = 1 << 20 // 1 MiB

	// tsDashboardBaseURL is the Tailscale admin console base URL.
	tsDashboardBaseURL = "https://login.tailscale.com/admin/machines"
)

// tailscaleCLICommand is the CLI binary name. Overridable in tests.
var tailscaleCLICommand = "tailscale"

// TailscaleFetcher defines the interface for retrieving Tailscale mesh status.
// Implementations include an API client (TailscaleAPIClient) and a CLI
// fallback (TailscaleCLIClient).
type TailscaleFetcher interface {
	FetchStatus(ctx context.Context) (*collectors.TailscaleStatus, error)
}

// ========== API Response Structs (unexported) ==========

// tsAPIDevicesResponse maps the Tailscale API /tailnet/{tailnet}/devices JSON.
type tsAPIDevicesResponse struct {
	Devices []tsAPIDevice `json:"devices"`
}

// tsAPIDevice represents a single device from the Tailscale API.
type tsAPIDevice struct {
	ID        string   `json:"id"`
	Hostname  string   `json:"hostname"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	OS        string   `json:"os"`
	Online    bool     `json:"online"`
	LastSeen  string   `json:"lastSeen"`
	Tags      []string `json:"tags"`
	NodeKey   string   `json:"nodeKey"`
}

// tsCLIStatusResponse maps the output of `tailscale status --json`.
type tsCLIStatusResponse struct {
	MagicDNSSuffix string                   `json:"MagicDNSSuffix"`
	Self           tsCLIPeer                `json:"Self"`
	Peer           map[string]tsCLIPeer     `json:"Peer"`
}

// tsCLIPeer represents a single peer from the CLI JSON output.
type tsCLIPeer struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	OS           string   `json:"OS"`
	Online       bool     `json:"Online"`
	LastSeen     string   `json:"LastSeen"`
	Tags         []string `json:"Tags"`
}

// ========== TailscaleAPIClient ==========

// TailscaleAPIClient fetches Tailscale mesh status via the HTTP API.
// It authenticates using Basic auth with the API key as the username
// and an empty password, per the Tailscale API convention.
type TailscaleAPIClient struct {
	tailnet    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
	baseURL    string // overridable for testing; empty uses production URL
}

// NewTailscaleAPIClient creates a TailscaleAPIClient with a 10-second HTTP timeout.
// If logger is nil, a no-op logger is used.
func NewTailscaleAPIClient(tailnet, apiKey string, logger *slog.Logger) *TailscaleAPIClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &TailscaleAPIClient{
		tailnet: tailnet,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: tsRequestTimeout,
		},
		logger: logger,
	}
}

// FetchStatus retrieves the list of devices in the tailnet from the Tailscale
// API and returns a populated TailscaleStatus. It uses Basic auth with the
// API key as the username.
func (c *TailscaleAPIClient) FetchStatus(ctx context.Context) (*collectors.TailscaleStatus, error) {
	url := c.devicesURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating tailscale API request: %w", err)
	}

	req.SetBasicAuth(c.apiKey, "")
	req.Header.Set("User-Agent", tsUserAgent)

	c.logger.Debug("fetching tailscale devices", "url", url, "tailnet", c.tailnet)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing tailscale API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, tsMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading tailscale API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &TSAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	var devicesResp tsAPIDevicesResponse
	if err := json.Unmarshal(body, &devicesResp); err != nil {
		return nil, fmt.Errorf("parsing tailscale devices JSON: %w", err)
	}

	nodes := make([]collectors.TailscaleNode, 0, len(devicesResp.Devices))
	onlineCount := 0

	for _, dev := range devicesResp.Devices {
		node := collectors.TailscaleNode{
			Name:         dev.Name,
			Hostname:     dev.Hostname,
			OS:           dev.OS,
			Online:       dev.Online,
			Tags:         dev.Tags,
			DashboardURL: fmt.Sprintf("%s/%s", tsDashboardBaseURL, dev.Hostname),
		}

		if len(dev.Addresses) > 0 {
			node.IP = dev.Addresses[0]
		}

		if dev.LastSeen != "" {
			if t, err := time.Parse(time.RFC3339, dev.LastSeen); err == nil {
				node.LastSeen = t
			} else {
				c.logger.Warn("failed to parse lastSeen timestamp",
					"hostname", dev.Hostname,
					"lastSeen", dev.LastSeen,
					"error", err,
				)
			}
		}

		if dev.Online {
			onlineCount++
		}

		nodes = append(nodes, node)
	}

	return &collectors.TailscaleStatus{
		Tailnet:     c.tailnet,
		OnlineCount: onlineCount,
		TotalCount:  len(nodes),
		Nodes:       nodes,
	}, nil
}

// devicesURL returns the devices endpoint URL, using baseURL for testing
// or the production URL by default.
func (c *TailscaleAPIClient) devicesURL() string {
	base := tsAPIBaseURL
	if c.baseURL != "" {
		base = c.baseURL
	}
	return fmt.Sprintf("%s/tailnet/%s/devices", base, c.tailnet)
}

// ========== TailscaleCLIClient ==========

// TailscaleCLIClient fetches Tailscale mesh status by running the tailscale
// CLI binary and parsing its JSON output. This serves as a fallback when the
// API key is not available or the API is unreachable.
type TailscaleCLIClient struct {
	logger *slog.Logger
}

// NewTailscaleCLIClient creates a TailscaleCLIClient.
// If logger is nil, a no-op logger is used.
func NewTailscaleCLIClient(logger *slog.Logger) *TailscaleCLIClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &TailscaleCLIClient{
		logger: logger,
	}
}

// FetchStatus runs `tailscale status --json` and parses the output into a
// TailscaleStatus. The Self node and all Peers are included in the result.
// MagicDNSSuffix is used as the tailnet name.
func (c *TailscaleCLIClient) FetchStatus(ctx context.Context) (*collectors.TailscaleStatus, error) {
	cmd := exec.CommandContext(ctx, tailscaleCLICommand, "status", "--json")

	c.logger.Debug("running tailscale CLI", "command", tailscaleCLICommand)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("tailscale CLI exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("running tailscale CLI: %w", err)
	}

	var cliResp tsCLIStatusResponse
	if err := json.Unmarshal(output, &cliResp); err != nil {
		return nil, fmt.Errorf("parsing tailscale CLI JSON: %w", err)
	}

	// Estimate capacity: Self + all peers.
	nodes := make([]collectors.TailscaleNode, 0, 1+len(cliResp.Peer))
	onlineCount := 0

	// Add the Self node.
	selfNode := c.peerToNode(cliResp.Self)
	nodes = append(nodes, selfNode)
	if selfNode.Online {
		onlineCount++
	}

	// Add all peers.
	for _, peer := range cliResp.Peer {
		node := c.peerToNode(peer)
		nodes = append(nodes, node)
		if node.Online {
			onlineCount++
		}
	}

	return &collectors.TailscaleStatus{
		Tailnet:     cliResp.MagicDNSSuffix,
		OnlineCount: onlineCount,
		TotalCount:  len(nodes),
		Nodes:       nodes,
	}, nil
}

// peerToNode converts a CLI peer entry to a TailscaleNode.
func (c *TailscaleCLIClient) peerToNode(peer tsCLIPeer) collectors.TailscaleNode {
	node := collectors.TailscaleNode{
		Name:         peer.DNSName,
		Hostname:     peer.HostName,
		OS:           peer.OS,
		Online:       peer.Online,
		Tags:         peer.Tags,
		DashboardURL: fmt.Sprintf("%s/%s", tsDashboardBaseURL, peer.HostName),
	}

	if len(peer.TailscaleIPs) > 0 {
		node.IP = peer.TailscaleIPs[0]
	}

	if peer.LastSeen != "" {
		if t, err := time.Parse(time.RFC3339, peer.LastSeen); err == nil {
			node.LastSeen = t
		} else {
			c.logger.Warn("failed to parse CLI lastSeen timestamp",
				"hostname", peer.HostName,
				"lastSeen", peer.LastSeen,
				"error", err,
			)
		}
	}

	return node
}

// ========== Error Types ==========

// TSAPIError represents a non-200 HTTP response from the Tailscale API.
type TSAPIError struct {
	StatusCode int
	Status     string
	Body       string
}

// Error returns a human-readable description of the API error.
func (e *TSAPIError) Error() string {
	return fmt.Sprintf("Tailscale API error: %s (body: %s)", e.Status, e.Body)
}
