package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// IPProfile contains opt-in metadata about the public egress IP. Provider
// fields are evidence, not a definitive fraud, blacklist, or safety verdict.
type IPProfile struct {
	Status         string              `json:"status"`
	Source         string              `json:"source"`
	Address        string              `json:"address,omitempty"`
	Type           string              `json:"type,omitempty"`
	ASN            string              `json:"asn,omitempty"`
	Organization   string              `json:"organization,omitempty"`
	ISP            string              `json:"isp,omitempty"`
	Domain         string              `json:"domain,omitempty"`
	Country        string              `json:"country,omitempty"`
	Region         string              `json:"region,omitempty"`
	City           string              `json:"city,omitempty"`
	Timezone       string              `json:"timezone,omitempty"`
	Classification string              `json:"classification,omitempty"`
	Risk           *RiskAssessment     `json:"risk,omitempty"`
	Security       *SecurityAssessment `json:"security,omitempty"`
	LatencyMS      int64               `json:"latency_ms,omitempty"`
	Error          string              `json:"error,omitempty"`
}

type RiskAssessment struct {
	Score   int      `json:"score"`
	Level   string   `json:"level"`
	Method  string   `json:"method"`
	Factors []string `json:"factors,omitempty"`
}

type SecurityAssessment struct {
	Anonymous bool `json:"anonymous"`
	Proxy     bool `json:"proxy"`
	VPN       bool `json:"vpn"`
	Tor       bool `json:"tor"`
	Hosting   bool `json:"hosting"`
}

type ipProfilePayload struct {
	IP         string `json:"ip"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	Connection struct {
		ASN    int    `json:"asn"`
		Org    string `json:"org"`
		ISP    string `json:"isp"`
		Domain string `json:"domain"`
	} `json:"connection"`
	Timezone struct {
		ID string `json:"id"`
	} `json:"timezone"`
	Security *SecurityAssessment `json:"security"`
}

func ipProfile(ctx context.Context, showIP bool) IPProfile {
	const target = "https://ipwho.is/"
	profile := IPProfile{Source: "ipwho.is"}
	start := now()
	request, err := newJSONRequest(ctx, target)
	if err != nil {
		profile.Status = "fail"
		profile.Error = "invalid profile target"
		return profile
	}
	response, err := newHTTPClient().Do(request)
	profile.LatencyMS = elapsedMS(start)
	if err != nil {
		profile.Status = statusForError(err)
		profile.Error = safeError(err)
		return profile
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		profile.Status = "fail"
		profile.Error = "provider returned " + response.Status
		return profile
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	if err != nil {
		profile.Status = "fail"
		profile.Error = "unable to read profile response"
		return profile
	}
	parsed, err := parseIPProfileResponse(body, showIP)
	if err != nil {
		profile.Status = "fail"
		profile.Error = "provider returned invalid profile"
		return profile
	}
	parsed.LatencyMS = profile.LatencyMS
	return parsed
}

func parseIPProfileResponse(body []byte, showIP bool) (IPProfile, error) {
	var payload ipProfilePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return IPProfile{}, err
	}
	address := net.ParseIP(strings.TrimSpace(payload.IP))
	if !payload.Success || address == nil {
		return IPProfile{}, fmt.Errorf("invalid profile: %s", payload.Message)
	}
	profile := IPProfile{
		Status:         "pass",
		Source:         "ipwho.is",
		Type:           payload.Type,
		ASN:            formatASN(payload.Connection.ASN),
		Organization:   payload.Connection.Org,
		ISP:            payload.Connection.ISP,
		Domain:         payload.Connection.Domain,
		Country:        payload.Country,
		Region:         payload.Region,
		City:           payload.City,
		Timezone:       payload.Timezone.ID,
		Classification: classifyNetwork(payload.Connection.Org, payload.Connection.ISP),
		Security:       payload.Security,
	}
	if showIP {
		profile.Address = address.String()
	}
	profile.Risk = assessRisk(profile.Classification, payload.Security)
	return profile, nil
}

func newJSONRequest(ctx context.Context, target string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "netdebug/"+versionForHeader())
	return request, nil
}

func formatASN(asn int) string {
	if asn <= 0 {
		return ""
	}
	return fmt.Sprintf("AS%d", asn)
}

func classifyNetwork(values ...string) string {
	value := strings.ToLower(strings.Join(values, " "))
	for _, keyword := range []string{
		"hosting", "host", "cloud", "server", "datacenter", "data center",
		"digitalocean", "amazon", "aws", "google cloud", "microsoft azure",
		"oracle cloud", "alibaba cloud", "tencent cloud", "vultr", "linode",
		"hetzner", "ovh", "contabo", "leaseweb",
	} {
		if strings.Contains(value, keyword) {
			return "hosting"
		}
	}
	if value != "" {
		return "isp-or-organization"
	}
	return "unknown"
}

func assessRisk(classification string, security *SecurityAssessment) *RiskAssessment {
	if security != nil {
		score := 0
		var factors []string
		if security.Proxy {
			score += 35
			factors = append(factors, "proxy")
		}
		if security.VPN {
			score += 30
			factors = append(factors, "vpn")
		}
		if security.Tor {
			score += 40
			factors = append(factors, "tor")
		}
		if security.Hosting {
			score += 20
			factors = append(factors, "hosting")
		}
		if security.Anonymous {
			score += 15
			factors = append(factors, "anonymous")
		}
		if score > 100 {
			score = 100
		}
		return &RiskAssessment{Score: score, Level: riskLevel(score), Method: "provider-flags", Factors: factors}
	}

	score := 0
	factors := []string{"provider threat flags unavailable"}
	if classification == "hosting" {
		score = 55
		factors = append(factors, "organization keyword suggests hosting/data-center network")
	}
	return &RiskAssessment{Score: score, Level: riskLevel(score), Method: "heuristic", Factors: factors}
}

func riskLevel(score int) string {
	switch {
	case score >= 80:
		return "very-high"
	case score >= 50:
		return "medium-high"
	case score >= 25:
		return "medium"
	default:
		return "low"
	}
}

// now is a small seam that keeps elapsed-time code easy to read and test.
func now() time.Time {
	return time.Now()
}
