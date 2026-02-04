package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// renderBillingContent renders the Billing tab content.
// It displays a summary of total spend, forecast, and budget status,
// followed by a per-provider table with detail links.
func renderBillingContent(data *collectors.BillingData, width, height int) string {
	if data == nil {
		return "No billing data available"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	spendStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	overBudgetStyle := lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
	underBudgetStyle := lipgloss.NewStyle().Foreground(colorSuccess)

	var sections []string

	// Section title.
	sections = append(sections, titleStyle.Render("Cloud Billing"))
	sections = append(sections, "")

	// Summary section.
	sections = append(sections, renderBillingSummary(
		data.Total, spendStyle, labelStyle, overBudgetStyle, underBudgetStyle,
	)...)
	sections = append(sections, "")

	// Provider table.
	if len(data.Providers) > 0 {
		sections = append(sections, renderProviderTable(data.Providers, width)...)
		sections = append(sections, "")

		// Provider detail links and fetch times.
		sections = append(sections, renderProviderDetails(data.Providers, labelStyle)...)
	}

	return strings.Join(sections, "\n")
}

// renderBillingSummary renders the aggregate billing totals.
func renderBillingSummary(
	total collectors.BillingSummary,
	spendStyle, labelStyle, overBudgetStyle, underBudgetStyle lipgloss.Style,
) []string {
	var lines []string

	// Current month total.
	lines = append(lines,
		labelStyle.Render("Current month: ")+
			spendStyle.Render(fmt.Sprintf("$%.2f", total.CurrentMonthUSD)),
	)

	// Forecast if available.
	if total.ForecastUSD != nil {
		lines = append(lines,
			labelStyle.Render("Forecast: ")+
				spendStyle.Render(fmt.Sprintf("$%.2f", *total.ForecastUSD)),
		)
	}

	// Budget status if available.
	if total.BudgetUSD != nil {
		budget := *total.BudgetUSD
		if total.CurrentMonthUSD > budget {
			over := total.CurrentMonthUSD - budget
			lines = append(lines,
				overBudgetStyle.Render(fmt.Sprintf("OVER BUDGET by $%.2f", over))+
					labelStyle.Render(fmt.Sprintf(" (budget: $%.2f)", budget)),
			)
		} else {
			remaining := budget - total.CurrentMonthUSD
			lines = append(lines,
				underBudgetStyle.Render(fmt.Sprintf("Under budget: $%.2f remaining", remaining))+
					labelStyle.Render(fmt.Sprintf(" (budget: $%.2f)", budget)),
			)
		}
	}

	return lines
}

// renderProviderTable renders the per-provider table using the table widget.
func renderProviderTable(providers []collectors.ProviderBilling, width int) []string {
	cfg := widgets.DefaultTableConfig()
	cfg.Columns = []widgets.Column{
		{Title: "Provider", Width: 14, Align: widgets.AlignLeft},
		{Title: "Status", Width: 12, Align: widgets.AlignLeft},
		{Title: "Spend", Width: 10, Align: widgets.AlignRight},
		{Title: "Forecast", Width: 10, Align: widgets.AlignRight},
		{Title: "Previous", Width: 10, Align: widgets.AlignRight},
		{Title: "Period", Width: 23, Align: widgets.AlignLeft},
	}
	if width > 0 {
		cfg.MaxWidth = width - 4
	}

	for _, p := range providers {
		status := widgets.RenderStatusFromString(p.Status)
		spend := fmt.Sprintf("$%.2f", p.CurrentMonth.SpendUSD)

		forecast := "-"
		if p.CurrentMonth.ForecastUSD != nil {
			forecast = fmt.Sprintf("$%.2f", *p.CurrentMonth.ForecastUSD)
		}

		previous := "-"
		if p.PreviousMonth != nil {
			previous = fmt.Sprintf("$%.2f", *p.PreviousMonth)
		}

		period := p.CurrentMonth.StartDate + " to " + p.CurrentMonth.EndDate

		row := []string{
			p.Provider,
			status,
			spend,
			forecast,
			previous,
			period,
		}
		cfg.Rows = append(cfg.Rows, row)
	}

	return []string{widgets.RenderTable(cfg)}
}

// renderProviderDetails renders dashboard links and fetch timestamps for each provider.
func renderProviderDetails(providers []collectors.ProviderBilling, labelStyle lipgloss.Style) []string {
	var lines []string

	for _, p := range providers {
		var detail strings.Builder

		if p.DashboardURL != "" {
			link := collectors.Link(p.DashboardURL, p.Provider+" dashboard")
			detail.WriteString(link)
		} else {
			detail.WriteString(p.Provider)
		}

		if !p.FetchedAt.IsZero() {
			detail.WriteString(labelStyle.Render(
				fmt.Sprintf("  (fetched %s)", p.FetchedAt.Format("15:04:05")),
			))
		}

		lines = append(lines, detail.String())
	}

	return lines
}
