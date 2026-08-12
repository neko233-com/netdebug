package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const schemaVersion = "1"

// Config controls probe scope. No probe sends user data in its request body.
type Config struct {
	Family      string
	ShowIP      bool
	PublicIP    bool
	Network     bool
	Timeout     time.Duration
	ToolVersion string
}

type Report struct {
	Schema      string   `json:"schema"`
	Tool        string   `json:"tool"`
	Version     string   `json:"version"`
	CollectedAt string   `json:"collected_at"`
	Platform    Platform `json:"platform"`
	Privacy     Privacy  `json:"privacy"`
	Network     Network  `json:"network"`
	PublicIPv4  *IPProbe `json:"public_ipv4,omitempty"`
	PublicIPv6  *IPProbe `json:"public_ipv6,omitempty"`
	DNS         []Probe  `json:"dns"`
	HTTPS       []Probe  `json:"https"`
	Summary     Summary  `json:"summary"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Privacy struct {
	Telemetry           bool     `json:"telemetry"`
	ReportUpload        bool     `json:"report_upload"`
	UserDataInRequests  bool     `json:"user_data_in_requests"`
	PublicIPProbeNotice string   `json:"public_ip_probe_notice"`
	Destinations        []string `json:"destinations"`
}

type Network struct {
	Interfaces int `json:"interfaces"`
	Up         int `json:"up"`
	Loopback   int `json:"loopback"`
}

type IPProbe struct {
	Status    string `json:"status"`
	Address   string `json:"address,omitempty"`
	Source    string `json:"source"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Probe struct {
	Target    string `json:"target"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Summary struct {
	Passed  int `json:"passed"`
	Warning int `json:"warning"`
	Failed  int `json:"failed"`
}

// Run executes local and outbound checks. It deliberately avoids hostnames,
// interface addresses, proxy credentials, report links, telemetry, and geo/IP
// reputation providers.
func Run(config Config) Report {
	if config.Family == "" {
		config.Family = "all"
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.ToolVersion == "" {
		config.ToolVersion = "dev"
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	report := Report{
		Schema:      schemaVersion,
		Tool:        "netdebug",
		Version:     config.ToolVersion,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Platform:    Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Privacy: Privacy{
			Telemetry:           false,
			ReportUpload:        false,
			UserDataInRequests:  false,
			PublicIPProbeNotice: "probe services can observe the transport source IP; no IP is sent as payload",
			Destinations: []string{
				"api4.ipify.org",
				"api6.ipify.org",
				"example.com",
				"www.cloudflare.com",
				"cloudflare.com",
				"example.net",
			},
		},
		Network: inspectInterfaces(),
		DNS:     []Probe{},
		HTTPS:   []Probe{},
	}
	if config.Network {
		report.DNS = []Probe{
			lookup(ctx, "example.com"),
			lookup(ctx, "cloudflare.com"),
		}
		report.HTTPS = []Probe{
			request(ctx, "https://www.cloudflare.com/cdn-cgi/trace"),
			request(ctx, "https://example.com/"),
		}
	} else {
		report.Privacy.Destinations = []string{}
	}

	if config.Network && config.PublicIP && config.Family != "6" {
		probe := publicIP(ctx, "https://api4.ipify.org?format=json", config.ShowIP)
		report.PublicIPv4 = &probe
	}
	if config.Network && config.PublicIP && config.Family != "4" {
		probe := publicIP(ctx, "https://api6.ipify.org?format=json", config.ShowIP)
		report.PublicIPv6 = &probe
	}
	report.Summary = summarize(report)
	return report
}

func inspectInterfaces() Network {
	interfaces, err := net.Interfaces()
	if err != nil {
		return Network{}
	}
	var network Network
	for _, iface := range interfaces {
		network.Interfaces++
		if iface.Flags&net.FlagUp != 0 {
			network.Up++
		}
		if iface.Flags&net.FlagLoopback != 0 {
			network.Loopback++
		}
	}
	return network
}

func lookup(ctx context.Context, target string) Probe {
	start := time.Now()
	addresses, err := net.DefaultResolver.LookupHost(ctx, target)
	probe := Probe{Target: target, LatencyMS: elapsedMS(start)}
	if err != nil {
		probe.Status = statusForError(err)
		probe.Error = safeError(err)
		return probe
	}
	probe.Status = "pass"
	probe.Detail = fmt.Sprintf("%d address(es)", len(addresses))
	return probe
}

func request(ctx context.Context, target string) Probe {
	start := time.Now()
	probe := Probe{Target: target, LatencyMS: elapsedMS(start)}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		probe.Status = "fail"
		probe.Error = "invalid probe target"
		return probe
	}
	request.Header.Set("User-Agent", "netdebug/"+versionForHeader())
	response, err := newHTTPClient().Do(request)
	probe.LatencyMS = elapsedMS(start)
	if err != nil {
		probe.Status = statusForError(err)
		probe.Error = safeError(err)
		return probe
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 4096)
	if response.StatusCode >= 200 && response.StatusCode < 400 {
		probe.Status = "pass"
		probe.Detail = response.Status
		return probe
	}
	probe.Status = "warn"
	probe.Detail = response.Status
	return probe
}

func publicIP(ctx context.Context, target string, showIP bool) IPProbe {
	start := time.Now()
	probe := IPProbe{Source: hostOf(target)}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		probe.Status = "fail"
		probe.Error = "invalid probe target"
		return probe
	}
	request.Header.Set("User-Agent", "netdebug/"+versionForHeader())
	response, err := newHTTPClient().Do(request)
	probe.LatencyMS = elapsedMS(start)
	if err != nil {
		probe.Status = statusForError(err)
		probe.Error = safeError(err)
		return probe
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		probe.Status = "fail"
		probe.Error = "probe returned " + response.Status
		return probe
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		probe.Status = "fail"
		probe.Error = "unable to read probe response"
		return probe
	}
	address, err := parseIPResponse(body)
	if err != nil {
		probe.Status = "fail"
		probe.Error = "probe returned invalid address"
		return probe
	}
	probe.Status = "pass"
	if showIP {
		probe.Address = address.String()
	}
	return probe
}

func parseIPResponse(body []byte) (net.IP, error) {
	var payload struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.IP != "" {
		if address := net.ParseIP(strings.TrimSpace(payload.IP)); address != nil {
			return address, nil
		}
	}
	address := net.ParseIP(strings.TrimSpace(string(body)))
	if address == nil {
		return nil, errors.New("invalid IP")
	}
	return address, nil
}

func summarize(report Report) Summary {
	var summary Summary
	add := func(status string) {
		switch status {
		case "pass":
			summary.Passed++
		case "warn", "skip":
			summary.Warning++
		default:
			summary.Failed++
		}
	}
	for _, probe := range report.DNS {
		add(probe.Status)
	}
	for _, probe := range report.HTTPS {
		add(probe.Status)
	}
	for _, probe := range []*IPProbe{report.PublicIPv4, report.PublicIPv6} {
		if probe != nil {
			add(probe.Status)
		}
	}
	return summary
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func statusForError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "warn"
	}
	return "fail"
}

func safeError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout"
	case strings.Contains(strings.ToLower(err.Error()), "no such host"):
		return "name resolution failed"
	default:
		return "connection failed"
	}
}

func hostOf(target string) string {
	for _, prefix := range []string{"https://", "http://"} {
		target = strings.TrimPrefix(target, prefix)
	}
	if index := strings.IndexByte(target, '/'); index >= 0 {
		target = target[:index]
	}
	return target
}

func newHTTPClient() *http.Client {
	return &http.Client{
		// Direct transport prevents inherited proxy URLs from forwarding
		// credentials or user-specific proxy metadata.
		Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true},
	}
}

func versionForHeader() string {
	return "0.1.0"
}
