package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newTestLogger returns a logger that writes to stderr at error level,
// keeping test output clean while still capturing warnings during debugging.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestAPIClient creates a TailscaleAPIClient pointed at a test server.
func newTestAPIClient(baseURL string) *TailscaleAPIClient {
	return &TailscaleAPIClient{
		tailnet:    "test-tailnet",
		apiKey:     "tskey-api-test-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		logger:     newTestLogger(),
	}
}

// ========== API Client Tests ==========

func TestTailscaleAPIClient_FetchStatus_Success(t *testing.T) {
	lastSeen := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Basic auth.
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic auth on request")
		}
		if user != "tskey-api-test-123" {
			t.Errorf("expected apiKey as username, got %q", user)
		}
		if pass != "" {
			t.Errorf("expected empty password, got %q", pass)
		}

		// Verify request method and path.
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		expectedPath := "/tailnet/test-tailnet/devices"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify User-Agent header.
		if got := r.Header.Get("User-Agent"); got != tsUserAgent {
			t.Errorf("expected User-Agent %q, got %q", tsUserAgent, got)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{
					ID:        "dev-1",
					Hostname:  "honey",
					Name:      "honey.tinyland.ts.net",
					Addresses: []string{"100.64.0.1", "fd7a:115c:a1e0::1"},
					OS:        "linux",
					Online:    true,
					LastSeen:  lastSeen.Format(time.RFC3339),
					Tags:      []string{"tag:server"},
					NodeKey:   "nodekey:abc123",
				},
				{
					ID:        "dev-2",
					Hostname:  "yoga",
					Name:      "yoga.tinyland.ts.net",
					Addresses: []string{"100.64.0.2"},
					OS:        "linux",
					Online:    true,
					LastSeen:  lastSeen.Format(time.RFC3339),
					NodeKey:   "nodekey:def456",
				},
				{
					ID:        "dev-3",
					Hostname:  "petting-zoo-mini",
					Name:      "petting-zoo-mini.tinyland.ts.net",
					Addresses: []string{"100.64.0.3"},
					OS:        "macOS",
					Online:    false,
					LastSeen:  lastSeen.Add(-24 * time.Hour).Format(time.RFC3339),
					NodeKey:   "nodekey:ghi789",
				},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Tailnet != "test-tailnet" {
		t.Errorf("expected Tailnet=test-tailnet, got %s", status.Tailnet)
	}
	if status.OnlineCount != 2 {
		t.Errorf("expected OnlineCount=2, got %d", status.OnlineCount)
	}
	if status.TotalCount != 3 {
		t.Errorf("expected TotalCount=3, got %d", status.TotalCount)
	}
	if len(status.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(status.Nodes))
	}

	// Verify first node details.
	node := status.Nodes[0]
	if node.Name != "honey.tinyland.ts.net" {
		t.Errorf("expected Name=honey.tinyland.ts.net, got %s", node.Name)
	}
	if node.Hostname != "honey" {
		t.Errorf("expected Hostname=honey, got %s", node.Hostname)
	}
	if node.IP != "100.64.0.1" {
		t.Errorf("expected IP=100.64.0.1, got %s", node.IP)
	}
	if node.OS != "linux" {
		t.Errorf("expected OS=linux, got %s", node.OS)
	}
	if !node.Online {
		t.Error("expected node to be online")
	}
	if !node.LastSeen.Equal(lastSeen) {
		t.Errorf("expected LastSeen=%v, got %v", lastSeen, node.LastSeen)
	}
	if len(node.Tags) != 1 || node.Tags[0] != "tag:server" {
		t.Errorf("expected Tags=[tag:server], got %v", node.Tags)
	}
	if node.DashboardURL != "https://login.tailscale.com/admin/machines/honey" {
		t.Errorf("expected DashboardURL for honey, got %s", node.DashboardURL)
	}

	// Verify offline node.
	offlineNode := status.Nodes[2]
	if offlineNode.Online {
		t.Error("expected petting-zoo-mini to be offline")
	}
}

func TestTailscaleAPIClient_FetchStatus_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	_, err := client.FetchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	apiErr, ok := err.(*TSAPIError)
	if !ok {
		t.Logf("error type: %T, message: %v", err, err)
		return
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected StatusCode=401, got %d", apiErr.StatusCode)
	}
}

func TestTailscaleAPIClient_FetchStatus_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	_, err := client.FetchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestTailscaleAPIClient_FetchStatus_ContextCancellation(t *testing.T) {
	// Use an already-cancelled context so the HTTP request fails immediately
	// without waiting for the server handler.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.FetchStatus(ctx)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestTailscaleAPIClient_FetchStatus_EmptyDevices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.TotalCount != 0 {
		t.Errorf("expected TotalCount=0, got %d", status.TotalCount)
	}
	if status.OnlineCount != 0 {
		t.Errorf("expected OnlineCount=0, got %d", status.OnlineCount)
	}
	if len(status.Nodes) != 0 {
		t.Errorf("expected empty nodes, got %d", len(status.Nodes))
	}
}

func TestTailscaleAPIClient_FetchStatus_NodeWithTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{
					ID:        "dev-tagged",
					Hostname:  "tagged-host",
					Name:      "tagged-host.tinyland.ts.net",
					Addresses: []string{"100.64.0.10"},
					OS:        "linux",
					Online:    true,
					LastSeen:  time.Now().UTC().Format(time.RFC3339),
					Tags:      []string{"tag:server", "tag:production", "tag:monitoring"},
				},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(status.Nodes))
	}

	node := status.Nodes[0]
	expectedTags := []string{"tag:server", "tag:production", "tag:monitoring"}
	if len(node.Tags) != len(expectedTags) {
		t.Fatalf("expected %d tags, got %d", len(expectedTags), len(node.Tags))
	}
	for i, tag := range expectedTags {
		if node.Tags[i] != tag {
			t.Errorf("expected tag[%d]=%s, got %s", i, tag, node.Tags[i])
		}
	}
}

func TestTailscaleAPIClient_FetchStatus_NoAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{
					ID:        "dev-noip",
					Hostname:  "noip-host",
					Name:      "noip-host.tinyland.ts.net",
					Addresses: []string{},
					OS:        "linux",
					Online:    false,
					LastSeen:  time.Now().UTC().Format(time.RFC3339),
				},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Nodes[0].IP != "" {
		t.Errorf("expected empty IP for node with no addresses, got %s", status.Nodes[0].IP)
	}
}

func TestTailscaleAPIClient_NilLogger(t *testing.T) {
	client := NewTailscaleAPIClient("test-net", "test-key", nil)
	if client.logger == nil {
		t.Fatal("expected non-nil logger even when nil is passed")
	}
}

// ========== CLI Client Tests ==========

// writeTestScript writes a shell script (or batch file on Windows) to tmpDir
// and returns the path. On unix, the script is made executable.
func writeTestScript(t *testing.T, tmpDir, scriptContent string) string {
	t.Helper()
	var scriptName, content string

	if runtime.GOOS == "windows" {
		scriptName = "tailscale.bat"
		content = scriptContent
	} else {
		scriptName = "tailscale"
		content = "#!/bin/sh\n" + scriptContent
	}

	scriptPath := filepath.Join(tmpDir, scriptName)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}
	return scriptPath
}

func TestTailscaleCLIClient_FetchStatus_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	selfLastSeen := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	peerLastSeen := time.Date(2026, 1, 20, 9, 30, 0, 0, time.UTC)

	cliOutput := tsCLIStatusResponse{
		MagicDNSSuffix: "tinyland.ts.net",
		Self: tsCLIPeer{
			HostName:     "yoga",
			DNSName:      "yoga.tinyland.ts.net.",
			TailscaleIPs: []string{"100.64.0.1", "fd7a:115c:a1e0::1"},
			OS:           "linux",
			Online:       true,
			LastSeen:     selfLastSeen.Format(time.RFC3339),
		},
		Peer: map[string]tsCLIPeer{
			"nodekey:abc123": {
				HostName:     "honey",
				DNSName:      "honey.tinyland.ts.net.",
				TailscaleIPs: []string{"100.64.0.2"},
				OS:           "linux",
				Online:       true,
				LastSeen:     peerLastSeen.Format(time.RFC3339),
				Tags:         []string{"tag:server"},
			},
			"nodekey:def456": {
				HostName:     "petting-zoo-mini",
				DNSName:      "petting-zoo-mini.tinyland.ts.net.",
				TailscaleIPs: []string{"100.64.0.3"},
				OS:           "macOS",
				Online:       false,
				LastSeen:     peerLastSeen.Add(-48 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	jsonBytes, err := json.Marshal(cliOutput)
	if err != nil {
		t.Fatalf("failed to marshal test CLI output: %v", err)
	}

	tmpDir := t.TempDir()
	scriptContent := fmt.Sprintf("cat <<'ENDOFJSON'\n%s\nENDOFJSON", string(jsonBytes))
	scriptPath := writeTestScript(t, tmpDir, scriptContent)

	// Override the CLI command to use our test script.
	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = scriptPath
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Tailnet != "tinyland.ts.net" {
		t.Errorf("expected Tailnet=tinyland.ts.net, got %s", status.Tailnet)
	}
	if status.TotalCount != 3 {
		t.Errorf("expected TotalCount=3, got %d", status.TotalCount)
	}
	if status.OnlineCount != 2 {
		t.Errorf("expected OnlineCount=2, got %d", status.OnlineCount)
	}

	// Verify the self node is included.
	foundSelf := false
	for _, node := range status.Nodes {
		if node.Hostname == "yoga" {
			foundSelf = true
			if node.IP != "100.64.0.1" {
				t.Errorf("expected Self IP=100.64.0.1, got %s", node.IP)
			}
			if !node.Online {
				t.Error("expected Self to be online")
			}
			if node.OS != "linux" {
				t.Errorf("expected Self OS=linux, got %s", node.OS)
			}
			if node.DashboardURL != "https://login.tailscale.com/admin/machines/yoga" {
				t.Errorf("expected DashboardURL for yoga, got %s", node.DashboardURL)
			}
		}
	}
	if !foundSelf {
		t.Error("Self node (yoga) not found in results")
	}
}

func TestTailscaleCLIClient_FetchStatus_CLINotFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = "/nonexistent/tailscale-not-found-binary"
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	_, err := client.FetchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error when CLI binary not found")
	}
}

func TestTailscaleCLIClient_FetchStatus_CLIError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	tmpDir := t.TempDir()
	scriptContent := `echo "tailscale daemon is not running" >&2
exit 1`
	scriptPath := writeTestScript(t, tmpDir, scriptContent)

	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = scriptPath
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	_, err := client.FetchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for non-zero CLI exit")
	}
}

func TestTailscaleCLIClient_FetchStatus_EmptyPeers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	cliOutput := tsCLIStatusResponse{
		MagicDNSSuffix: "solo.ts.net",
		Self: tsCLIPeer{
			HostName:     "lonely-node",
			DNSName:      "lonely-node.solo.ts.net.",
			TailscaleIPs: []string{"100.64.0.1"},
			OS:           "linux",
			Online:       true,
			LastSeen:     time.Now().UTC().Format(time.RFC3339),
		},
		Peer: map[string]tsCLIPeer{},
	}

	jsonBytes, err := json.Marshal(cliOutput)
	if err != nil {
		t.Fatalf("failed to marshal test CLI output: %v", err)
	}

	tmpDir := t.TempDir()
	scriptContent := fmt.Sprintf("cat <<'ENDOFJSON'\n%s\nENDOFJSON", string(jsonBytes))
	scriptPath := writeTestScript(t, tmpDir, scriptContent)

	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = scriptPath
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Tailnet != "solo.ts.net" {
		t.Errorf("expected Tailnet=solo.ts.net, got %s", status.Tailnet)
	}
	if status.TotalCount != 1 {
		t.Errorf("expected TotalCount=1 (Self only), got %d", status.TotalCount)
	}
	if status.OnlineCount != 1 {
		t.Errorf("expected OnlineCount=1, got %d", status.OnlineCount)
	}
	if len(status.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(status.Nodes))
	}
	if status.Nodes[0].Hostname != "lonely-node" {
		t.Errorf("expected Hostname=lonely-node, got %s", status.Nodes[0].Hostname)
	}
}

func TestTailscaleCLIClient_FetchStatus_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	// Use an already-cancelled context so exec.CommandContext fails immediately.
	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = "/bin/sh"
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.FetchStatus(ctx)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestTailscaleCLIClient_NilLogger(t *testing.T) {
	client := NewTailscaleCLIClient(nil)
	if client.logger == nil {
		t.Fatal("expected non-nil logger even when nil is passed")
	}
}

// ========== Online/Offline Counting Tests ==========

func TestOnlineOfflineCounting_AllOnline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{Hostname: "a", Online: true, Addresses: []string{"100.64.0.1"}},
				{Hostname: "b", Online: true, Addresses: []string{"100.64.0.2"}},
				{Hostname: "c", Online: true, Addresses: []string{"100.64.0.3"}},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.OnlineCount != 3 {
		t.Errorf("expected OnlineCount=3, got %d", status.OnlineCount)
	}
	if status.TotalCount != 3 {
		t.Errorf("expected TotalCount=3, got %d", status.TotalCount)
	}
}

func TestOnlineOfflineCounting_AllOffline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{Hostname: "a", Online: false, Addresses: []string{"100.64.0.1"}},
				{Hostname: "b", Online: false, Addresses: []string{"100.64.0.2"}},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.OnlineCount != 0 {
		t.Errorf("expected OnlineCount=0, got %d", status.OnlineCount)
	}
	if status.TotalCount != 2 {
		t.Errorf("expected TotalCount=2, got %d", status.TotalCount)
	}
}

func TestOnlineOfflineCounting_Mixed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{Hostname: "a", Online: true, Addresses: []string{"100.64.0.1"}},
				{Hostname: "b", Online: false, Addresses: []string{"100.64.0.2"}},
				{Hostname: "c", Online: true, Addresses: []string{"100.64.0.3"}},
				{Hostname: "d", Online: false, Addresses: []string{"100.64.0.4"}},
				{Hostname: "e", Online: true, Addresses: []string{"100.64.0.5"}},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.OnlineCount != 3 {
		t.Errorf("expected OnlineCount=3, got %d", status.OnlineCount)
	}
	if status.TotalCount != 5 {
		t.Errorf("expected TotalCount=5, got %d", status.TotalCount)
	}
}

// ========== Dashboard URL Generation Tests ==========

func TestDashboardURL_API(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tsAPIDevicesResponse{
			Devices: []tsAPIDevice{
				{Hostname: "my-server", Online: true, Addresses: []string{"100.64.0.1"}},
				{Hostname: "my-laptop", Online: true, Addresses: []string{"100.64.0.2"}},
			},
		})
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"my-server": "https://login.tailscale.com/admin/machines/my-server",
		"my-laptop": "https://login.tailscale.com/admin/machines/my-laptop",
	}

	for _, node := range status.Nodes {
		want, ok := expected[node.Hostname]
		if !ok {
			t.Errorf("unexpected hostname: %s", node.Hostname)
			continue
		}
		if node.DashboardURL != want {
			t.Errorf("DashboardURL for %s: got %s, want %s", node.Hostname, node.DashboardURL, want)
		}
	}
}

func TestDashboardURL_CLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CLI tests use shell scripts, skipping on Windows")
	}

	cliOutput := tsCLIStatusResponse{
		MagicDNSSuffix: "test.ts.net",
		Self: tsCLIPeer{
			HostName:     "dev-box",
			DNSName:      "dev-box.test.ts.net.",
			TailscaleIPs: []string{"100.64.0.1"},
			OS:           "linux",
			Online:       true,
		},
		Peer: map[string]tsCLIPeer{},
	}

	jsonBytes, err := json.Marshal(cliOutput)
	if err != nil {
		t.Fatalf("failed to marshal test CLI output: %v", err)
	}

	tmpDir := t.TempDir()
	scriptContent := fmt.Sprintf("cat <<'ENDOFJSON'\n%s\nENDOFJSON", string(jsonBytes))
	scriptPath := writeTestScript(t, tmpDir, scriptContent)

	oldCmd := tailscaleCLICommand
	tailscaleCLICommand = scriptPath
	defer func() { tailscaleCLICommand = oldCmd }()

	client := NewTailscaleCLIClient(newTestLogger())
	status, err := client.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(status.Nodes))
	}

	wantURL := "https://login.tailscale.com/admin/machines/dev-box"
	if status.Nodes[0].DashboardURL != wantURL {
		t.Errorf("DashboardURL = %s, want %s", status.Nodes[0].DashboardURL, wantURL)
	}
}

// ========== Interface Compliance ==========

func TestTailscaleAPIClient_ImplementsFetcher(t *testing.T) {
	var _ TailscaleFetcher = (*TailscaleAPIClient)(nil)
}

func TestTailscaleCLIClient_ImplementsFetcher(t *testing.T) {
	var _ TailscaleFetcher = (*TailscaleCLIClient)(nil)
}

// ========== TSAPIError Tests ==========

func TestTSAPIError_Error(t *testing.T) {
	err := &TSAPIError{
		StatusCode: 403,
		Status:     "403 Forbidden",
		Body:       `{"message":"access denied"}`,
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	if got := msg; got != `Tailscale API error: 403 Forbidden (body: {"message":"access denied"})` {
		t.Errorf("unexpected error message: %s", got)
	}
}

// ========== Body Read Limit Test ==========

func TestTailscaleAPIClient_BodyReadLimit(t *testing.T) {
	// Respond with a body that exceeds tsMaxResponseBytes (1 MiB).
	// The client should still handle this gracefully (truncated read
	// leads to JSON parse failure).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 2 MiB of data.
		data := make([]byte, 2<<20)
		for i := range data {
			data[i] = 'x'
		}
		w.Write(data)
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL)
	_, err := client.FetchStatus(context.Background())
	// Should fail with a JSON parse error since the body is truncated/invalid.
	if err == nil {
		t.Fatal("expected error for oversized response body")
	}
}

