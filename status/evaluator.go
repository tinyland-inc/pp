package status

import (
	"fmt"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Level represents system health.
type Level int

const (
	LevelHealthy  Level = iota // Everything normal
	LevelWarning               // Something needs attention
	LevelCritical              // Immediate attention needed
	LevelUnknown               // Insufficient data
)

// String returns the human-readable name for a Level.
func (l Level) String() string {
	switch l {
	case LevelHealthy:
		return "healthy"
	case LevelWarning:
		return "warning"
	case LevelCritical:
		return "critical"
	case LevelUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// levelSeverity returns the sort order for levels. Higher is worse.
// Critical > Warning > Unknown > Healthy.
func levelSeverity(l Level) int {
	switch l {
	case LevelHealthy:
		return 0
	case LevelUnknown:
		return 1
	case LevelWarning:
		return 2
	case LevelCritical:
		return 3
	default:
		return 0
	}
}

// worstLevel returns whichever Level is more severe.
func worstLevel(a, b Level) Level {
	if levelSeverity(a) >= levelSeverity(b) {
		return a
	}
	return b
}

// ComponentStatus holds the evaluation result for a single component.
type ComponentStatus struct {
	Component string // "claude", "billing", "infra"
	Level     Level
	Reason    string // Human-readable reason
}

// SystemStatus is the aggregate evaluation result.
type SystemStatus struct {
	Overall     Level             // Worst of all components
	Components  []ComponentStatus
	EvaluatedAt time.Time
}

// EvaluatorConfig holds thresholds for evaluation rules.
type EvaluatorConfig struct {
	// Claude thresholds
	ClaudeWarningPercent  float64 // Default: 80.0
	ClaudeCriticalPercent float64 // Default: 95.0

	// Billing thresholds
	BillingBudgetWarningPercent  float64 // Default: 80.0 (% of budget)
	BillingBudgetCriticalPercent float64 // Default: 100.0

	// Infra thresholds
	TailscaleWarningPercent float64 // Default: 50.0 (% online)
	K8sNodeReadyMinPercent  float64 // Default: 80.0
}

// DefaultEvaluatorConfig returns an EvaluatorConfig with sensible defaults.
func DefaultEvaluatorConfig() EvaluatorConfig {
	return EvaluatorConfig{
		ClaudeWarningPercent:         80.0,
		ClaudeCriticalPercent:        95.0,
		BillingBudgetWarningPercent:  80.0,
		BillingBudgetCriticalPercent: 100.0,
		TailscaleWarningPercent:      50.0,
		K8sNodeReadyMinPercent:       80.0,
	}
}

// Evaluator analyzes collector data and determines system health.
type Evaluator struct {
	config EvaluatorConfig
}

// NewEvaluator creates an Evaluator with the given configuration.
func NewEvaluator(cfg EvaluatorConfig) *Evaluator {
	return &Evaluator{config: cfg}
}

// Evaluate runs all evaluation rules and returns the aggregate status.
func (e *Evaluator) Evaluate(claude *collectors.ClaudeUsage, billing *collectors.BillingData, infra *collectors.InfraStatus) SystemStatus {
	claudeStatus := e.evaluateClaude(claude)
	billingStatus := e.evaluateBilling(billing)
	infraStatus := e.evaluateInfra(infra)

	components := []ComponentStatus{claudeStatus, billingStatus, infraStatus}

	overall := components[0].Level
	for _, c := range components[1:] {
		overall = worstLevel(overall, c.Level)
	}

	return SystemStatus{
		Overall:     overall,
		Components:  components,
		EvaluatedAt: time.Now(),
	}
}

// evaluateClaude checks Claude account usage levels.
func (e *Evaluator) evaluateClaude(data *collectors.ClaudeUsage) ComponentStatus {
	if data == nil {
		return ComponentStatus{
			Component: "claude",
			Level:     LevelUnknown,
			Reason:    "no data",
		}
	}

	resultLevel := LevelHealthy
	resultReason := "all accounts normal"

	for _, acct := range data.Accounts {
		// Check account status first.
		if acct.Status != "ok" {
			candidate := LevelWarning
			reason := fmt.Sprintf("account %s status: %s", acct.Name, acct.Status)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		}

		// Check 5-hour utilization for subscription accounts.
		if acct.FiveHour != nil {
			util := acct.FiveHour.Utilization
			if util > e.config.ClaudeCriticalPercent {
				candidate := LevelCritical
				reason := fmt.Sprintf("account %s 5h usage at %.0f%%", acct.Name, util)
				if levelSeverity(candidate) > levelSeverity(resultLevel) {
					resultLevel = candidate
					resultReason = reason
				}
			} else if util > e.config.ClaudeWarningPercent {
				candidate := LevelWarning
				reason := fmt.Sprintf("account %s 5h usage at %.0f%%", acct.Name, util)
				if levelSeverity(candidate) > levelSeverity(resultLevel) {
					resultLevel = candidate
					resultReason = reason
				}
			}
		}
	}

	return ComponentStatus{
		Component: "claude",
		Level:     resultLevel,
		Reason:    resultReason,
	}
}

// evaluateBilling checks billing against budgets and forecasts.
func (e *Evaluator) evaluateBilling(data *collectors.BillingData) ComponentStatus {
	if data == nil {
		return ComponentStatus{
			Component: "billing",
			Level:     LevelUnknown,
			Reason:    "no data",
		}
	}

	resultLevel := LevelHealthy
	resultReason := fmt.Sprintf("spend $%.2f this month", data.Total.CurrentMonthUSD)

	// Check provider statuses.
	for _, p := range data.Providers {
		if p.Status == "error" {
			candidate := LevelWarning
			reason := fmt.Sprintf("provider %s status: error", p.Provider)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		}
	}

	// Check budget thresholds on total.
	if data.Total.BudgetUSD != nil {
		budget := *data.Total.BudgetUSD
		spend := data.Total.CurrentMonthUSD

		criticalThreshold := budget * e.config.BillingBudgetCriticalPercent / 100.0
		warningThreshold := budget * e.config.BillingBudgetWarningPercent / 100.0

		if spend > criticalThreshold {
			candidate := LevelCritical
			reason := fmt.Sprintf("spend $%.2f exceeds budget $%.2f", spend, budget)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		} else if spend > warningThreshold {
			candidate := LevelWarning
			reason := fmt.Sprintf("spend $%.2f approaching budget $%.2f", spend, budget)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		}

		// Check forecast against budget.
		if data.Total.ForecastUSD != nil {
			forecast := *data.Total.ForecastUSD
			if forecast > budget {
				candidate := LevelWarning
				reason := fmt.Sprintf("forecast $%.2f exceeds budget $%.2f", forecast, budget)
				if levelSeverity(candidate) > levelSeverity(resultLevel) {
					resultLevel = candidate
					resultReason = reason
				}
			}
		}
	}

	return ComponentStatus{
		Component: "billing",
		Level:     resultLevel,
		Reason:    resultReason,
	}
}

// evaluateInfra checks Tailscale connectivity and K8s health.
func (e *Evaluator) evaluateInfra(data *collectors.InfraStatus) ComponentStatus {
	if data == nil {
		return ComponentStatus{
			Component: "infra",
			Level:     LevelUnknown,
			Reason:    "no data",
		}
	}

	resultLevel := LevelHealthy
	resultReason := "infrastructure healthy"

	// Evaluate Tailscale.
	if data.Tailscale != nil {
		ts := data.Tailscale
		if ts.TotalCount > 0 {
			if ts.OnlineCount == 0 {
				candidate := LevelCritical
				reason := fmt.Sprintf("all %d tailscale nodes offline", ts.TotalCount)
				if levelSeverity(candidate) > levelSeverity(resultLevel) {
					resultLevel = candidate
					resultReason = reason
				}
			} else {
				onlinePct := float64(ts.OnlineCount) / float64(ts.TotalCount) * 100.0
				if onlinePct < e.config.TailscaleWarningPercent {
					candidate := LevelWarning
					reason := fmt.Sprintf("tailscale %d/%d nodes online (%.0f%%)", ts.OnlineCount, ts.TotalCount, onlinePct)
					if levelSeverity(candidate) > levelSeverity(resultLevel) {
						resultLevel = candidate
						resultReason = reason
					}
				}
			}
		}
	}

	// Evaluate Kubernetes clusters.
	for _, cluster := range data.Kubernetes {
		if cluster.Status == "offline" {
			candidate := LevelCritical
			reason := fmt.Sprintf("k8s cluster %s offline", cluster.Name)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		} else if cluster.Status == "degraded" {
			candidate := LevelWarning
			reason := fmt.Sprintf("k8s cluster %s degraded", cluster.Name)
			if levelSeverity(candidate) > levelSeverity(resultLevel) {
				resultLevel = candidate
				resultReason = reason
			}
		}

		// Check node readiness.
		if cluster.TotalNodes > 0 {
			readyPct := float64(cluster.ReadyNodes) / float64(cluster.TotalNodes) * 100.0
			if readyPct < e.config.K8sNodeReadyMinPercent {
				candidate := LevelWarning
				reason := fmt.Sprintf("k8s cluster %s: %d/%d nodes ready (%.0f%%)", cluster.Name, cluster.ReadyNodes, cluster.TotalNodes, readyPct)
				if levelSeverity(candidate) > levelSeverity(resultLevel) {
					resultLevel = candidate
					resultReason = reason
				}
			}
		}
	}

	return ComponentStatus{
		Component: "infra",
		Level:     resultLevel,
		Reason:    resultReason,
	}
}
