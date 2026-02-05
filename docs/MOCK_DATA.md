# Mock Data System

**Purpose**: Provide consistent zero-state mock data for all collector types to ensure UI initialization is predictable and testing is straightforward.

## Overview

The mock data system (`collectors/mocks.go`) provides default zero-state data for all collector types:

- **ClaudeUsage** - Claude account usage data
- **BillingData** - Billing information across cloud providers
- **InfraStatus** - Tailscale nodes + Kubernetes clusters
- **FastfetchData** - System information

## Available Mock Functions

### Basic Mocks (Zero-State)

These return data structures with realistic shapes but zero/minimal values:

```go
// Mock Claude usage with single healthy account at 0% utilization
claude := collectors.MockClaudeUsage()

// Mock billing with $0 spend and 30 days of zero-spend history
billing := collectors.MockBillingData()

// Mock infrastructure with 3 offline Tailscale nodes (no metrics)
infra := collectors.MockInfraStatus()

// Mock system information with basic hardware details
fastfetch := collectors.MockFastfetchData()
```

### Enhanced Mocks (With Data)

These return data structures populated with realistic test values:

```go
// Mock billing with realistic 30-day spending pattern
// Shows gradual increase then plateau (~$220/month total)
billing := collectors.MockBillingDataWithHistory()

// Mock infrastructure with 3 online nodes WITH metrics
// All nodes show CPU/RAM/Disk percentages for gauge testing
infra := collectors.MockInfraStatusWithMetrics()
```

## Usage Examples

### 1. Testing UI Components

```go
func TestBannerWithMockData(t *testing.T) {
    // Use zero-state mocks for testing empty state UI
    claude := collectors.MockClaudeUsage()
    billing := collectors.MockBillingData()
    infra := collectors.MockInfraStatus()

    // Generate banner
    banner := NewMockBanner(cfg, claude, billing, infra, nil)
    output, err := banner.Generate()

    // Verify UI handles zero-state gracefully
    assert.NoError(t, err)
    assert.Contains(t, output, "ts: 0/3 online")
    assert.Contains(t, output, "$0 this month")
}
```

### 2. Testing Visual Features

```go
func TestGaugeRendering(t *testing.T) {
    // Use enhanced mocks with actual metrics
    infra := collectors.MockInfraStatusWithMetrics()

    // Should have 3 online nodes with metrics
    assert.Equal(t, 3, infra.Tailscale.OnlineCount)

    // All nodes should have CPU/RAM/Disk values
    for _, node := range infra.Tailscale.Nodes {
        assert.NotNil(t, node.CPUPercent)
        assert.NotNil(t, node.RAMPercent)
        assert.NotNil(t, node.DiskPercent)
    }
}
```

### 3. Testing Sparkline Rendering

```go
func TestSparklineWithHistory(t *testing.T) {
    // Use billing mock with 30-day history
    billing := collectors.MockBillingDataWithHistory()

    // Should have history for sparklines
    assert.NotNil(t, billing.History)
    assert.Equal(t, 30, len(billing.History.TotalHistory))

    // Verify spend increases over time (not flat line)
    first := billing.History.TotalHistory[0].SpendUSD
    last := billing.History.TotalHistory[29].SpendUSD
    assert.Greater(t, last, first)
}
```

## Demo Tool

A command-line demo tool is available to visualize mock data in different layout modes:

```bash
# Build the demo tool
go build -o demo-mocks ./cmd/demo-mocks/

# Compact layout (80x24) with zero-state data
./demo-mocks -width 80 -height 24

# Wide layout (160x60) with node metrics
./demo-mocks -width 160 -height 60 -with-metrics

# UltraWide (200x80) with billing history sparklines
./demo-mocks -width 200 -height 80 -with-history

# Combine both enhanced features
./demo-mocks -width 200 -height 80 -with-metrics -with-history
```

### Demo Tool Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-width` | int | 160 | Terminal width in characters |
| `-height` | int | 60 | Terminal height in lines |
| `-with-metrics` | bool | false | Use MockInfraStatusWithMetrics() |
| `-with-history` | bool | false | Use MockBillingDataWithHistory() |

## Mock Data Characteristics

### MockClaudeUsage()

- 1 account: "primary" (subscription, pro tier)
- 0% utilization (5-hour and 7-day windows)
- Status: `ok`
- Reset times: 2 hours (5h), 48 hours (7d)

### MockBillingData()

- 2 providers: civo, digitalocean
- Current spend: $0.00 each
- 30 days of history: all zeros (flat sparkline)
- No budget or forecast set

### MockBillingDataWithHistory()

- 2 providers: civo (~$143), digitalocean (~$82)
- Total: ~$225 current month
- 30 days of history: gradual increase, then plateau
- Realistic sparkline visualization

### MockInfraStatus()

- Tailscale: 3 nodes, all offline, no metrics
- Nodes: yoga, xoxd-bates, petting-zoo-mini
- Kubernetes: 1 cluster (tinyland-civo-dev, status unknown)
- Total: 0/3 nodes online

### MockInfraStatusWithMetrics()

- Tailscale: 3 nodes, all online, WITH metrics
- Node metrics:
  - yoga: CPU 45%, RAM 67%, Disk 32%
  - xoxd-bates: CPU 23%, RAM 89%, Disk 56%
  - petting-zoo-mini: CPU 78%, RAM 45%, Disk 91%
- Kubernetes: 1 healthy cluster, 2 nodes, 39/41 pods running

### MockFastfetchData()

- OS: Rocky Linux 10.1
- Kernel: 6.12.0
- CPU: Intel i7-8550U
- RAM: 4.5 GiB / 15.4 GiB
- Disk: 100 GiB / 237 GiB
- Uptime: 1 day, 2 hours

## Integration with Real Collectors

Mock data follows the exact same structure as real collector output, making it a drop-in replacement:

```go
// In production
claude := claudeCollector.Collect(ctx)

// In testing/demo
claude := collectors.MockClaudeUsage()

// Both return *collectors.ClaudeUsage - same type!
```

## Performance

Mock generation is fast (<1µs per mock):

```
BenchmarkMockClaudeUsage-8               5000000    237 ns/op
BenchmarkMockBillingData-8               2000000    623 ns/op
BenchmarkMockBillingDataWithHistory-8     500000   2847 ns/op
BenchmarkMockInfraStatus-8               3000000    412 ns/op
BenchmarkMockFastfetchData-8             5000000    198 ns/op
```

## When to Use Each Mock

| Scenario | Mock Function | Reason |
|----------|---------------|--------|
| Testing empty state UI | Basic mocks | Verify graceful handling of no data |
| Testing error messages | Basic mocks | Clean slate for error scenarios |
| Testing gauge rendering | `MockInfraStatusWithMetrics()` | Need CPU/RAM/Disk values |
| Testing sparklines | `MockBillingDataWithHistory()` | Need trend data |
| Visual regression tests | Enhanced mocks | Want fully populated UI |
| Demo/documentation | Enhanced mocks | Show all features working |

## Future Enhancements

Possible additions to the mock system:

1. **Mock with errors**: Return data with Status="error" for error handling tests
2. **Mock with warnings**: Return data triggering warning thresholds
3. **Mock with critical state**: Return data triggering critical alerts
4. **Configurable mocks**: Allow customizing mock parameters (e.g., set specific CPU %)
5. **Randomized mocks**: Generate realistic random data for fuzz testing

## See Also

- `collectors/mocks.go` - Mock data implementation
- `collectors/mocks_test.go` - Mock data tests
- `cmd/demo-mocks/` - Demo tool source code
- `display/banner/mock.go` - MockBanner uses mocks for testing
