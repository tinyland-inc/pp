package infra

import (
	"fmt"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Known dashboard URLs for cloud providers and services.
const (
	tailscaleAdminURL     = "https://login.tailscale.com/admin/machines"
	civoDashboardURL      = "https://dashboard.civo.com"
	doDashboardURL        = "https://cloud.digitalocean.com/account/billing"
	awsDashboardURL       = "https://console.aws.amazon.com/billing/home"
	dreamhostDashboardURL = "https://panel.dreamhost.com/index.cgi?tree=billing.account"
)

// providerDashboardURLs maps provider names to their billing dashboard URLs.
var providerDashboardURLs = map[string]string{
	"civo":         civoDashboardURL,
	"digitalocean": doDashboardURL,
	"aws":          awsDashboardURL,
	"dreamhost":    dreamhostDashboardURL,
}

// DashboardLinks holds OSC 8 hyperlinks for all infrastructure components.
type DashboardLinks struct {
	// Tailscale is an OSC 8 link to the Tailscale admin console.
	Tailscale string

	// Kubernetes holds OSC 8 links for each cluster dashboard.
	Kubernetes []string

	// Billing holds OSC 8 links for each billing provider dashboard.
	Billing []string
}

// GenerateTailscaleLink returns an OSC 8 hyperlink to the Tailscale admin
// console. The link text shows the online/total node count (e.g., "ts:3/5").
// Returns an empty string if status is nil.
func GenerateTailscaleLink(status *collectors.TailscaleStatus) string {
	if status == nil {
		return ""
	}

	text := fmt.Sprintf("ts:%d/%d", status.OnlineCount, status.TotalCount)
	return collectors.Link(tailscaleAdminURL, text)
}

// GenerateK8sLinks returns OSC 8 hyperlinks for each Kubernetes cluster that
// has a DashboardURL configured. The link text shows the cluster name and
// status (e.g., "civo-prod:healthy"). Clusters without a DashboardURL are
// skipped.
func GenerateK8sLinks(clusters []collectors.KubernetesCluster) []string {
	if len(clusters) == 0 {
		return nil
	}

	var links []string
	for _, cluster := range clusters {
		if cluster.DashboardURL == "" {
			continue
		}
		text := fmt.Sprintf("%s:%s", cluster.Name, cluster.Status)
		links = append(links, collectors.Link(cluster.DashboardURL, text))
	}

	return links
}

// GenerateBillingLinks returns OSC 8 hyperlinks for each billing provider.
// The link text shows the provider name and current month spend (e.g.,
// "civo:$12.50"). If a provider has a DashboardURL set, that is used;
// otherwise, the known URL from providerDashboardURLs is used as a fallback.
func GenerateBillingLinks(billing *collectors.BillingData) []string {
	if billing == nil || len(billing.Providers) == 0 {
		return nil
	}

	var links []string
	for _, provider := range billing.Providers {
		url := provider.DashboardURL
		if url == "" {
			url = providerDashboardURLs[provider.Provider]
		}
		if url == "" {
			continue
		}

		text := fmt.Sprintf("%s:$%.2f", provider.Provider, provider.CurrentMonth.SpendUSD)
		links = append(links, collectors.Link(url, text))
	}

	return links
}

// GenerateAllLinks creates a complete DashboardLinks struct from all available
// infrastructure and billing data.
func GenerateAllLinks(
	tsStatus *collectors.TailscaleStatus,
	clusters []collectors.KubernetesCluster,
	billing *collectors.BillingData,
) DashboardLinks {
	return DashboardLinks{
		Tailscale:  GenerateTailscaleLink(tsStatus),
		Kubernetes: GenerateK8sLinks(clusters),
		Billing:    GenerateBillingLinks(billing),
	}
}
