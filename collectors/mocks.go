package collectors

import (
	"time"
)

// MockClaudeUsage returns zero-state Claude usage data with a single example account.
// Useful for UI initialization and testing without real API calls.
func MockClaudeUsage() *ClaudeUsage {
	now := time.Now()
	fiveHourReset := now.Add(2 * time.Hour) // 2 hours until 5-hour window resets
	sevenDayReset := now.Add(48 * time.Hour) // 2 days until 7-day window resets

	return &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:      "primary",
				Type:      "subscription",
				Tier:      "pro",
				Status:    StatusOK,
				ShortName: "main",
				Priority:  1,
				FiveHour: &UsagePeriod{
					Utilization: 0.0,
					ResetsAt:    fiveHourReset,
				},
				SevenDay: &UsagePeriod{
					Utilization: 0.0,
					ResetsAt:    sevenDayReset,
				},
				ExtraUsage: &ExtraUsage{
					Enabled:      true,
					MonthlyLimit: 10000, // $100.00
					UsedCredits:  0.0,
					Utilization:  0.0,
				},
			},
		},
	}
}

// MockBillingData returns zero-state billing data with example providers.
// Shows $0 spend across all providers with empty history for sparklines.
func MockBillingData() *BillingData {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endDate := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())

	// Generate 30 days of zero spend history for sparkline consistency
	var totalHistory []DailySpend
	for i := 0; i < 30; i++ {
		date := startDate.AddDate(0, 0, -30+i)
		totalHistory = append(totalHistory, DailySpend{
			Date:     date.Format("2006-01-02"),
			SpendUSD: 0.0,
		})
	}

	return &BillingData{
		Providers: []ProviderBilling{
			{
				Provider:     "civo",
				AccountName:  "tinyland",
				Status:       StatusOK,
				DashboardURL: "https://dashboard.civo.com",
				CurrentMonth: MonthCost{
					SpendUSD:  0.0,
					StartDate: startDate.Format("2006-01-02"),
					EndDate:   endDate.Format("2006-01-02"),
				},
				FetchedAt: now,
			},
			{
				Provider:     "digitalocean",
				AccountName:  "main",
				Status:       StatusOK,
				DashboardURL: "https://cloud.digitalocean.com",
				CurrentMonth: MonthCost{
					SpendUSD:  0.0,
					StartDate: startDate.Format("2006-01-02"),
					EndDate:   endDate.Format("2006-01-02"),
				},
				FetchedAt: now,
			},
		},
		Total: BillingSummary{
			CurrentMonthUSD: 0.0,
			SuccessCount:    2,
			ErrorCount:      0,
			TotalConfigured: 2,
		},
		History: &BillingHistory{
			ProviderHistory: map[string][]DailySpend{
				"civo":         totalHistory,
				"digitalocean": totalHistory,
			},
			TotalHistory: totalHistory,
			LastUpdated:  now,
		},
	}
}

// MockInfraStatus returns zero-state infrastructure data with example nodes.
// Shows a Tailscale mesh with nodes but no metrics (all nil).
func MockInfraStatus() *InfraStatus {
	now := time.Now()

	return &InfraStatus{
		Tailscale: &TailscaleStatus{
			Tailnet:     "example.ts.net",
			OnlineCount: 0,
			TotalCount:  3,
			Nodes: []TailscaleNode{
				{
					Name:         "yoga",
					Hostname:     "yoga",
					IP:           "100.64.0.1",
					OS:           "linux",
					Online:       false,
					LastSeen:     now.Add(-10 * time.Minute),
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   nil,
					RAMPercent:   nil,
					DiskPercent:  nil,
				},
				{
					Name:         "xoxd-bates",
					Hostname:     "xoxd-bates",
					IP:           "100.64.0.2",
					OS:           "darwin",
					Online:       false,
					LastSeen:     now.Add(-5 * time.Minute),
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   nil,
					RAMPercent:   nil,
					DiskPercent:  nil,
				},
				{
					Name:         "petting-zoo-mini",
					Hostname:     "petting-zoo-mini",
					IP:           "100.64.0.3",
					OS:           "darwin",
					Online:       false,
					LastSeen:     now.Add(-2 * time.Minute),
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   nil,
					RAMPercent:   nil,
					DiskPercent:  nil,
				},
			},
		},
		Kubernetes: []KubernetesCluster{
			{
				Name:         "tinyland-civo-dev",
				Context:      "tinyland-civo-dev",
				Platform:     "civo",
				Status:       "unknown",
				APIEndpoint:  "",
				DashboardURL: "https://dashboard.civo.com",
				Nodes:        []KubernetesNode{},
				TotalNodes:   0,
				ReadyNodes:   0,
				TotalPods:    0,
				RunningPods:  0,
				Version:      "",
			},
		},
	}
}

// MockInfraStatusWithMetrics returns infrastructure data with example node metrics.
// Useful for testing gauge rendering and metric display features.
func MockInfraStatusWithMetrics() *InfraStatus {
	now := time.Now()

	cpu1, ram1, disk1 := 45.0, 67.0, 32.0
	cpu2, ram2, disk2 := 23.0, 89.0, 56.0
	cpu3, ram3, disk3 := 78.0, 45.0, 91.0

	return &InfraStatus{
		Tailscale: &TailscaleStatus{
			Tailnet:     "example.ts.net",
			OnlineCount: 3,
			TotalCount:  3,
			Nodes: []TailscaleNode{
				{
					Name:         "yoga",
					Hostname:     "yoga",
					IP:           "100.64.0.1",
					OS:           "linux",
					Online:       true,
					LastSeen:     now,
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   &cpu1,
					RAMPercent:   &ram1,
					DiskPercent:  &disk1,
				},
				{
					Name:         "xoxd-bates",
					Hostname:     "xoxd-bates",
					IP:           "100.64.0.2",
					OS:           "darwin",
					Online:       true,
					LastSeen:     now,
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   &cpu2,
					RAMPercent:   &ram2,
					DiskPercent:  &disk2,
				},
				{
					Name:         "petting-zoo-mini",
					Hostname:     "petting-zoo-mini",
					IP:           "100.64.0.3",
					OS:           "darwin",
					Online:       true,
					LastSeen:     now,
					DashboardURL: "https://login.tailscale.com/admin/machines",
					CPUPercent:   &cpu3,
					RAMPercent:   &ram3,
					DiskPercent:  &disk3,
				},
			},
		},
		Kubernetes: []KubernetesCluster{
			{
				Name:         "tinyland-civo-dev",
				Context:      "tinyland-civo-dev",
				Platform:     "civo",
				Status:       "healthy",
				APIEndpoint:  "https://api.civo.com",
				DashboardURL: "https://dashboard.civo.com",
				Nodes: []KubernetesNode{
					{
						Name:       "node-1",
						Status:     "Ready",
						CPUPercent: 45.0,
						MemPercent: 67.0,
						PodCount:   23,
						MaxPods:    110,
					},
					{
						Name:       "node-2",
						Status:     "Ready",
						CPUPercent: 32.0,
						MemPercent: 54.0,
						PodCount:   18,
						MaxPods:    110,
					},
				},
				TotalNodes:   2,
				ReadyNodes:   2,
				TotalPods:    41,
				RunningPods:  39,
				Version:      "v1.28.3",
			},
		},
	}
}

// MockFastfetchData returns zero-state system information data.
// Shows basic system info without desktop environment details.
func MockFastfetchData() *FastfetchData {
	return &FastfetchData{
		OS: FastfetchModule{
			Type:   "OS",
			Result: "Rocky Linux 10.1",
		},
		Host: FastfetchModule{
			Type:   "Host",
			Result: "Lenovo Yoga",
		},
		Kernel: FastfetchModule{
			Type:   "Kernel",
			Result: "6.12.0",
		},
		Uptime: FastfetchModule{
			Type:   "Uptime",
			Result: "1 day, 2 hours",
		},
		CPU: FastfetchModule{
			Type:   "CPU",
			Result: "Intel i7-8550U",
		},
		Memory: FastfetchModule{
			Type:   "Memory",
			Result: "4.5 GiB / 15.4 GiB",
		},
		Disk: FastfetchModule{
			Type:   "Disk",
			Result: "100 GiB / 237 GiB",
		},
		Shell: FastfetchModule{
			Type:   "Shell",
			Result: "fish 3.7.0",
		},
		Terminal: FastfetchModule{
			Type:   "Terminal",
			Result: "alacritty",
		},
		LocalIP: FastfetchModule{
			Type:   "Local IP",
			Result: "192.168.1.100",
		},
	}
}

// MockSysMetricsData returns zero-state system metrics with empty history.
// Shows moderate utilization values suitable for UI testing.
func MockSysMetricsData() *SysMetricsData {
	// Generate a 60-sample history with a gentle sine-like pattern.
	cpuHistory := make([]float64, 60)
	ramHistory := make([]float64, 60)
	diskHistory := make([]float64, 60)
	for i := 0; i < 60; i++ {
		// CPU: oscillates around 35% (20-50 range)
		cpuHistory[i] = 35.0 + 15.0*float64(i%20)/20.0
		// RAM: slowly increases from 40% to 55%
		ramHistory[i] = 40.0 + float64(i)*0.25
		// Disk: essentially static around 43%
		diskHistory[i] = 43.0 + float64(i%5)*0.1
	}

	return &SysMetricsData{
		CPU:         32.5,
		RAM:         52.3,
		Disk:        43.1,
		LoadAvg1:    1.25,
		LoadAvg5:    0.98,
		LoadAvg15:   0.75,
		CPUHistory:  cpuHistory,
		RAMHistory:  ramHistory,
		DiskHistory: diskHistory,
	}
}

// MockBillingDataWithHistory returns billing data with 30 days of realistic spend patterns.
// Shows gradual increase throughout the month, useful for sparkline visualization testing.
func MockBillingDataWithHistory() *BillingData {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endDate := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())

	// Generate realistic 30-day spend pattern (gradual increase, then plateau)
	var totalHistory []DailySpend
	var civoHistory []DailySpend
	var doHistory []DailySpend

	for i := 0; i < 30; i++ {
		date := startDate.AddDate(0, 0, -30+i)

		// Civo: starts low, increases, plateaus at ~$140
		civoSpend := 95.0 + float64(i)*2.5
		if civoSpend > 143.0 {
			civoSpend = 143.0 + float64(i%10)*0.02
		}

		// DigitalOcean: starts low, increases, plateaus at ~$80
		doSpend := 45.0 + float64(i)*1.5
		if doSpend > 82.0 {
			doSpend = 82.0 + float64(i%10)*0.05
		}

		civoHistory = append(civoHistory, DailySpend{
			Date:     date.Format("2006-01-02"),
			SpendUSD: civoSpend,
		})

		doHistory = append(doHistory, DailySpend{
			Date:     date.Format("2006-01-02"),
			SpendUSD: doSpend,
		})

		totalHistory = append(totalHistory, DailySpend{
			Date:     date.Format("2006-01-02"),
			SpendUSD: civoSpend + doSpend,
		})
	}

	// Current month totals (latest day from history)
	civoTotal := civoHistory[len(civoHistory)-1].SpendUSD
	doTotal := doHistory[len(doHistory)-1].SpendUSD

	return &BillingData{
		Providers: []ProviderBilling{
			{
				Provider:     "civo",
				AccountName:  "tinyland",
				Status:       StatusOK,
				DashboardURL: "https://dashboard.civo.com",
				CurrentMonth: MonthCost{
					SpendUSD:  civoTotal,
					StartDate: startDate.Format("2006-01-02"),
					EndDate:   endDate.Format("2006-01-02"),
				},
				FetchedAt: now,
			},
			{
				Provider:     "digitalocean",
				AccountName:  "main",
				Status:       StatusOK,
				DashboardURL: "https://cloud.digitalocean.com",
				CurrentMonth: MonthCost{
					SpendUSD:  doTotal,
					StartDate: startDate.Format("2006-01-02"),
					EndDate:   endDate.Format("2006-01-02"),
				},
				FetchedAt: now,
			},
		},
		Total: BillingSummary{
			CurrentMonthUSD: civoTotal + doTotal,
			SuccessCount:    2,
			ErrorCount:      0,
			TotalConfigured: 2,
		},
		History: &BillingHistory{
			ProviderHistory: map[string][]DailySpend{
				"civo":         civoHistory,
				"digitalocean": doHistory,
			},
			TotalHistory: totalHistory,
			LastUpdated:  now,
		},
	}
}
