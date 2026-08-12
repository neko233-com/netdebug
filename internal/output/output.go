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

type translations struct {
	subtitle, privacy, noTelemetry, noUpload, hidden                                           string
	network, interfaces, total, up, loopback                                                   string
	publicIPv4, publicIPv6, ipProfile, optIn                                                   string
	provider, publicIP, asn, organization, location, typeLabel, risk, threatFlags, unavailable string
	dns, https, summary, passed, warning, failed, footer                                       string
	platform, collected, telemetry, reportUpload, userData, disabled, no, localOnly            string
	statusPass, statusWarn, statusFail, available                                              string
	result, detail, latency, field, value, status, privacyNotice                               string
	endpoints, noProbes, version, method, score, checks                                        string
}

func translationsFor(language string) translations {
	if strings.EqualFold(strings.TrimSpace(language), "cn") {
		return translations{
			subtitle: "隐私优先网络诊断", privacy: "隐私", noTelemetry: "无遥测", noUpload: "不上传报告", hidden: "IP 默认隐藏",
			network: "网络", interfaces: "网卡", total: "总计", up: "在线", loopback: "回环",
			publicIPv4: "公网 IPv4", publicIPv6: "公网 IPv6", ipProfile: "IP 情报（主动开启）", optIn: "主动开启",
			provider: "数据源", publicIP: "公网 IP", asn: "ASN", organization: "组织", location: "位置", typeLabel: "类型",
			risk: "风险", threatFlags: "威胁标记", unavailable: "数据源未提供", dns: "DNS", https: "HTTPS",
			summary: "摘要", passed: "通过", warning: "警告", failed: "失败",
			footer:   "仅访问固定公网端点；结果不会发送到任何报告服务。",
			platform: "平台", collected: "采集时间", telemetry: "遥测", reportUpload: "报告上传", userData: "请求携带用户数据",
			disabled: "禁用", no: "否", localOnly: "仅本机统计",
			statusPass: "✓ 通过", statusWarn: "! 警告", statusFail: "× 失败", available: "可用（已隐藏）",
			result: "结果", detail: "详情", latency: "延迟", field: "字段", value: "值", status: "状态",
			endpoints: "固定端点", noProbes: "暂无探针结果", version: "版本", method: "方法", score: "评分", checks: "检查项",
			privacyNotice: "公网 IP 探针可观察传输源 IP；netdebug 不把 IP 作为请求数据发送、不存储结果，也不创建报告链接。",
		}
	}
	return translations{
		subtitle: "privacy-first network diagnostics", privacy: "Privacy", noTelemetry: "no telemetry",
		noUpload: "no report upload", hidden: "IP hidden by default",
		network: "Network", interfaces: "Interfaces", total: "total", up: "up", loopback: "loopback",
		publicIPv4: "Public IPv4", publicIPv6: "Public IPv6", ipProfile: "IP Profile", optIn: "opt-in",
		provider: "Provider", publicIP: "Public IP", asn: "ASN", organization: "Organization", location: "Location",
		typeLabel: "Type", risk: "Risk", threatFlags: "Threat flags", unavailable: "unavailable from provider",
		dns: "DNS", https: "HTTPS", summary: "Summary", passed: "passed", warning: "warning", failed: "failed",
		footer:   "Only fixed public endpoints are contacted; no results are sent anywhere.",
		platform: "Platform", collected: "Collected", telemetry: "Telemetry", reportUpload: "Report upload",
		userData: "User data in requests", disabled: "disabled", no: "no", localOnly: "local counts only",
		statusPass: "✓ pass", statusWarn: "! warn", statusFail: "× fail", available: "available (hidden)",
		result: "Result", detail: "Detail", latency: "Latency", field: "Field", value: "Value", status: "Status",
		endpoints: "Fixed endpoints", noProbes: "no probe results", version: "Version", method: "Method", score: "Score", checks: "Checks",
		privacyNotice: "Public IP probe services can observe the transport source IP. netdebug does not send IP as request data, store results, or create report links.",
	}
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
	text := translationsFor(report.Language)
	c := func(code, value string) string {
		if !color {
			return value
		}
		return "\033[" + code + "m" + value + "\033[0m"
	}
	line := strings.Repeat("─", 66)
	fmt.Fprintln(writer, c("36", "╭"+line+"╮"))
	fmt.Fprintf(writer, "│ %s  %s\n", c("1;36", "netdebug"), c("2", text.subtitle))
	fmt.Fprintf(writer, "│ %s/%s · %s: %s · %s: %s\n", report.Platform.OS, report.Platform.Arch, text.version, emptyDash(report.Version), text.collected, emptyDash(report.CollectedAt))
	fmt.Fprintln(writer, c("36", "╰"+line+"╯"))

	consoleSection(writer, c, text.privacy)
	fmt.Fprintf(writer, "  %s  %s  %s\n", statusBadge("pass", text, c), text.noTelemetry, text.noUpload)
	fmt.Fprintf(writer, "  %s · %s: %d\n", text.hidden, text.endpoints, len(report.Privacy.Destinations))

	consoleSection(writer, c, text.network)
	fmt.Fprintf(writer, "  %-12s %d %s · %d %s · %d %s\n", text.interfaces, report.Network.Interfaces, text.total, report.Network.Up, text.up, report.Network.Loopback, text.loopback)
	if report.PublicIPv4 != nil {
		fmt.Fprintf(writer, "  %-12s %-11s %s\n", text.publicIPv4, statusBadge(report.PublicIPv4.Status, text, c), hiddenAddressText(report.PublicIPv4, text))
	}
	if report.PublicIPv6 != nil {
		fmt.Fprintf(writer, "  %-12s %-11s %s\n", text.publicIPv6, statusBadge(report.PublicIPv6.Status, text, c), hiddenAddressText(report.PublicIPv6, text))
	}
	if report.IPProfile != nil {
		profile := report.IPProfile
		consoleSection(writer, c, text.ipProfile+" · "+text.optIn)
		fmt.Fprintf(writer, "  %-12s %s  %s\n", text.provider, profile.Source, statusBadge(profile.Status, text, c))
		fmt.Fprintf(writer, "  %-12s %s\n", text.publicIP, profileAddressText(profile, text))
		fmt.Fprintf(writer, "  %-12s %s · %s\n", text.asn, emptyDash(profile.ASN), emptyDash(profile.Organization))
		fmt.Fprintf(writer, "  %-12s %s\n", text.location, profileLocation(profile))
		fmt.Fprintf(writer, "  %-12s %s\n", text.typeLabel, translateType(profile.Classification, report.Language))
		if profile.Risk != nil {
			score := clampScore(profile.Risk.Score)
			fmt.Fprintf(writer, "  %-12s %s %d/100 · %s\n", text.risk, riskBar(score, c), score, translateRisk(profile.Risk.Level, report.Language))
			fmt.Fprintf(writer, "  %-12s %s\n", text.method, profile.Risk.Method)
		}
		if profile.Security == nil {
			fmt.Fprintf(writer, "  %-12s %s\n", text.threatFlags, text.unavailable)
		} else {
			fmt.Fprintf(writer, "  %-12s proxy=%t vpn=%t tor=%t hosting=%t\n", text.threatFlags, profile.Security.Proxy, profile.Security.VPN, profile.Security.Tor, profile.Security.Hosting)
		}
	}

	consoleSection(writer, c, text.dns)
	if len(report.DNS) == 0 {
		fmt.Fprintf(writer, "  %s\n", c("2", text.noProbes))
	}
	for _, probe := range report.DNS {
		fmt.Fprintf(writer, "  %-28s %-11s %4d ms", probe.Target, statusBadge(probe.Status, text, c), probe.LatencyMS)
		if probe.Detail != "" {
			fmt.Fprintf(writer, " · %s", probe.Detail)
		}
		fmt.Fprintln(writer)
	}

	consoleSection(writer, c, text.https)
	if len(report.HTTPS) == 0 {
		fmt.Fprintf(writer, "  %s\n", c("2", text.noProbes))
	}
	for _, probe := range report.HTTPS {
		fmt.Fprintf(writer, "  %-42s %-11s %4d ms", probe.Target, statusBadge(probe.Status, text, c), probe.LatencyMS)
		if probe.Detail != "" {
			fmt.Fprintf(writer, " · %s", probe.Detail)
		}
		fmt.Fprintln(writer)
	}

	consoleSection(writer, c, text.summary)
	fmt.Fprintf(writer, "  %s %d   %s %d   %s %d\n", c("32", "✓ "+text.passed), report.Summary.Passed, c("33", "! "+text.warning), report.Summary.Warning, c("31", "× "+text.failed), report.Summary.Failed)
	fmt.Fprintf(writer, "\n%s\n", c("2", text.footer))
}

func consoleSection(writer io.Writer, color func(string, string) string, title string) {
	fmt.Fprintf(writer, "\n%s %s\n", color("1;36", "◆"), color("1;36", title))
	fmt.Fprintf(writer, "  %s\n", color("2", "────────────────────────────────────────────────────────────"))
}

func renderMarkdown(writer io.Writer, report diagnostics.Report) error {
	text := translationsFor(report.Language)
	var b strings.Builder
	fmt.Fprintf(&b, "# netdebug\n\n%s.\n\n", text.subtitle)
	fmt.Fprintf(&b, "- %s: %s\n- %s: %s\n- %s: %s\n- %s: %s\n- %s: %s\n\n",
		text.platform, code(report.Platform.OS+"/"+report.Platform.Arch),
		text.collected, code(report.CollectedAt),
		text.telemetry, code(text.disabled),
		text.reportUpload, code(text.disabled),
		text.userData, code(text.no))
	fmt.Fprintf(&b, "## %s\n\n", text.network)
	fmt.Fprintf(&b, "| %s | %s | %s |\n|---|---:|---|\n", text.field, text.result, text.detail)
	fmt.Fprintf(&b, "| %s | %d %s / %d %s / %d %s | %s |\n", text.interfaces, report.Network.Interfaces, text.total, report.Network.Up, text.up, report.Network.Loopback, text.loopback, text.localOnly)
	if report.PublicIPv4 != nil {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", text.publicIPv4, statusText(report.PublicIPv4.Status, text), hiddenAddressText(report.PublicIPv4, text))
	}
	if report.PublicIPv6 != nil {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", text.publicIPv6, statusText(report.PublicIPv6.Status, text), hiddenAddressText(report.PublicIPv6, text))
	}
	if report.IPProfile != nil {
		profile := report.IPProfile
		fmt.Fprintf(&b, "\n## %s · %s\n\n| %s | %s |\n|---|---|\n", text.ipProfile, text.optIn, text.field, text.value)
		fmt.Fprintf(&b, "| %s | %s / %s |\n", text.provider, code(profile.Source), statusText(profile.Status, text))
		fmt.Fprintf(&b, "| %s | %s |\n", text.publicIP, profileAddressText(profile, text))
		fmt.Fprintf(&b, "| %s / %s | %s / %s |\n", text.asn, text.organization, code(profile.ASN), emptyDash(profile.Organization))
		fmt.Fprintf(&b, "| %s | %s |\n", text.location, profileLocation(profile))
		fmt.Fprintf(&b, "| %s | %s |\n", text.typeLabel, code(translateType(profile.Classification, report.Language)))
		if profile.Risk != nil {
			fmt.Fprintf(&b, "| %s | %s · %s · %s |\n", text.risk, code(fmt.Sprintf("%d/100", profile.Risk.Score)), code(translateRisk(profile.Risk.Level, report.Language)), code(profile.Risk.Method))
		}
	}
	fmt.Fprintf(&b, "\n## %s\n\n| %s | %s | %s | %s |\n|---|---|---:|---|\n", text.dns, text.field, text.status, text.latency, text.detail)
	for _, probe := range report.DNS {
		fmt.Fprintf(&b, "| %s | %s | %d ms | %s |\n", code(probe.Target), statusText(probe.Status, text), probe.LatencyMS, emptyDash(probe.Detail))
	}
	fmt.Fprintf(&b, "\n## %s\n\n| %s | %s | %s | %s |\n|---|---|---:|---|\n", text.https, text.field, text.status, text.latency, text.detail)
	for _, probe := range report.HTTPS {
		fmt.Fprintf(&b, "| %s | %s | %d ms | %s |\n", code(probe.Target), statusText(probe.Status, text), probe.LatencyMS, emptyDash(probe.Detail))
	}
	fmt.Fprintf(&b, "\n## %s\n\n%d %s · %d %s · %d %s\n\n> %s\n", text.summary, report.Summary.Passed, text.passed, report.Summary.Warning, text.warning, report.Summary.Failed, text.failed, text.privacyNotice)
	_, err := io.WriteString(writer, b.String())
	return err
}

func code(value string) string {
	return string(rune(96)) + value + string(rune(96))
}

func statusText(value string, text translations) string {
	switch value {
	case "pass":
		return strings.TrimSpace(text.statusPass)
	case "warn":
		return strings.TrimSpace(text.statusWarn)
	default:
		return strings.TrimSpace(text.statusFail)
	}
}

func statusLabel(value string, text translations, color func(string, string) string) string {
	switch value {
	case "pass":
		return color("32", text.statusPass)
	case "warn":
		return color("33", text.statusWarn)
	default:
		return color("31", text.statusFail)
	}
}

func statusBadge(value string, text translations, color func(string, string) string) string {
	return color(statusColor(value), "["+statusLabel(value, text, func(_ string, value string) string { return value })+"]")
}

func statusColor(value string) string {
	switch value {
	case "pass":
		return "32"
	case "warn":
		return "33"
	default:
		return "31"
	}
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func riskBar(score int, color func(string, string) string) string {
	score = clampScore(score)
	filled := (score*20 + 50) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	return color(statusColorForScore(score), bar)
}

func statusColorForScore(score int) string {
	switch {
	case score >= 80:
		return "31"
	case score >= 50:
		return "33"
	default:
		return "32"
	}
}

func hiddenAddress(probe *diagnostics.IPProbe) string {
	return hiddenAddressText(probe, translationsFor(""))
}

func hiddenAddressText(probe *diagnostics.IPProbe, text translations) string {
	if probe.Address != "" {
		return probe.Address
	}
	if probe.Status == "pass" {
		return text.available
	}
	return emptyDash(probe.Error)
}

func profileAddress(profile *diagnostics.IPProfile) string {
	return profileAddressText(profile, translationsFor(""))
}

func profileAddressText(profile *diagnostics.IPProfile, text translations) string {
	if profile.Address != "" {
		return profile.Address
	}
	if profile.Status == "pass" {
		return text.available
	}
	return emptyDash(profile.Error)
}

func profileLocation(profile *diagnostics.IPProfile) string {
	parts := make([]string, 0, 3)
	for _, value := range []string{profile.City, profile.Region, profile.Country} {
		if value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func translateType(value, language string) string {
	if strings.EqualFold(strings.TrimSpace(language), "cn") {
		switch value {
		case "hosting":
			return "托管机房"
		case "residential":
			return "住宅"
		case "business":
			return "企业"
		}
	}
	return emptyDash(value)
}

func translateRisk(value, language string) string {
	if !strings.EqualFold(strings.TrimSpace(language), "cn") {
		return emptyDash(value)
	}
	switch value {
	case "low":
		return "低风险"
	case "medium":
		return "中风险"
	case "medium-high":
		return "中高风险"
	case "high":
		return "高风险"
	default:
		return emptyDash(value)
	}
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
