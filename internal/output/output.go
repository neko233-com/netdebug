package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/neko233-com/netdebug/internal/diagnostics"
)

type Options struct {
	Format string
	Color  bool
}

func Render(writer io.Writer, report diagnostics.Report, options Options) error {
	switch options.Format {
	case "json":
		return renderJSON(writer, report)
	case "markdown":
		return renderMarkdown(writer, report)
	default:
		renderConsole(writer, report, options.Color)
		return nil
	}
}

func renderJSON(writer io.Writer, report diagnostics.Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func renderConsole(writer io.Writer, report diagnostics.Report, color bool) {
	c := func(code, value string) string {
		if !color {
			return value
		}
		return "\033[" + code + "m" + value + "\033[0m"
	}
	status := func(value string) string {
		switch value {
		case "pass":
			return c("32", "✓ pass")
		case "warn":
			return c("33", "! warn")
		default:
			return c("31", "× fail")
		}
	}

	line := "────────────────────────────────────────────────────────────"
	fmt.Fprintln(writer, c("36", "╭"+line+"╮"))
	fmt.Fprintf(writer, "│ %s  %s\n", c("1;36", "netdebug"), c("2", "privacy-first network diagnostics"))
	fmt.Fprintf(writer, "│ %s · %s · %s\n", report.Platform.OS, report.Platform.Arch, report.CollectedAt)
	fmt.Fprintln(writer, c("36", "╰"+line+"╯"))
	fmt.Fprintf(writer, "\n%s  no telemetry · no report upload · IP hidden by default\n", c("2", "Privacy"))

	fmt.Fprintf(writer, "\n%s\n", c("1;36", "Network"))
	fmt.Fprintf(writer, "  Interfaces   %d total · %d up · %d loopback\n", report.Network.Interfaces, report.Network.Up, report.Network.Loopback)
	if report.PublicIPv4 != nil {
		fmt.Fprintf(writer, "  Public IPv4  %s · %s\n", status(report.PublicIPv4.Status), hiddenAddress(report.PublicIPv4))
	}
	if report.PublicIPv6 != nil {
		fmt.Fprintf(writer, "  Public IPv6  %s · %s\n", status(report.PublicIPv6.Status), hiddenAddress(report.PublicIPv6))
	}

	fmt.Fprintf(writer, "\n%s\n", c("1;36", "DNS"))
	for _, probe := range report.DNS {
		fmt.Fprintf(writer, "  %-20s %s  %d ms", probe.Target, status(probe.Status), probe.LatencyMS)
		if probe.Detail != "" {
			fmt.Fprintf(writer, " · %s", probe.Detail)
		}
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "\n%s\n", c("1;36", "HTTPS"))
	for _, probe := range report.HTTPS {
		fmt.Fprintf(writer, "  %-42s %s  %d ms", probe.Target, status(probe.Status), probe.LatencyMS)
		if probe.Detail != "" {
			fmt.Fprintf(writer, " · %s", probe.Detail)
		}
		fmt.Fprintln(writer)
	}

	fmt.Fprintf(writer, "\n%s  %s %d  %s %d  %s %d\n", c("1;36", "Summary"), c("32", "passed"), report.Summary.Passed, c("33", "warning"), report.Summary.Warning, c("31", "failed"), report.Summary.Failed)
	fmt.Fprintf(writer, "%s\n", c("2", "Only fixed public endpoints are contacted; no results are sent anywhere."))
}

func renderMarkdown(writer io.Writer, report diagnostics.Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# netdebug\n\nPrivacy-first network diagnostics.\n\n")
	fmt.Fprintf(&b, "- Platform: `%s/%s`\n- Collected: `%s`\n- Telemetry: `disabled`\n- Report upload: `disabled`\n- User data in requests: `no`\n\n", report.Platform.OS, report.Platform.Arch, report.CollectedAt)
	b.WriteString("## Network\n\n")
	b.WriteString("| Check | Result | Detail |\n|---|---:|---|\n")
	fmt.Fprintf(&b, "| Interfaces | %d total / %d up / %d loopback | local counts only |\n", report.Network.Interfaces, report.Network.Up, report.Network.Loopback)
	if report.PublicIPv4 != nil {
		fmt.Fprintf(&b, "| Public IPv4 | %s | %s |\n", report.PublicIPv4.Status, hiddenAddress(report.PublicIPv4))
	}
	if report.PublicIPv6 != nil {
		fmt.Fprintf(&b, "| Public IPv6 | %s | %s |\n", report.PublicIPv6.Status, hiddenAddress(report.PublicIPv6))
	}
	b.WriteString("\n## DNS\n\n| Target | Status | Latency | Detail |\n|---|---|---:|---|\n")
	for _, probe := range report.DNS {
		fmt.Fprintf(&b, "| `%s` | %s | %d ms | %s |\n", probe.Target, probe.Status, probe.LatencyMS, emptyDash(probe.Detail))
	}
	b.WriteString("\n## HTTPS\n\n| Target | Status | Latency | Detail |\n|---|---|---:|---|\n")
	for _, probe := range report.HTTPS {
		fmt.Fprintf(&b, "| `%s` | %s | %d ms | %s |\n", probe.Target, probe.Status, probe.LatencyMS, emptyDash(probe.Detail))
	}
	fmt.Fprintf(&b, "\n## Summary\n\n`%d` passed · `%d` warning · `%d` failed\n\n> Public IP probe services can observe transport source IP. netdebug does not send IP as request data, store results, or create report links.\n", report.Summary.Passed, report.Summary.Warning, report.Summary.Failed)
	_, err := io.WriteString(writer, b.String())
	return err
}

func hiddenAddress(probe *diagnostics.IPProbe) string {
	if probe.Address != "" {
		return probe.Address
	}
	if probe.Status == "pass" {
		return "available (hidden)"
	}
	return emptyDash(probe.Error)
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
