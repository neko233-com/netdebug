package diagnostics

import (
	"fmt"
	"net"
	"regexp"
	"testing"
)

func TestReportTimestampFormat(t *testing.T) {
	report := Run(Config{Network: false})
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}$`).MatchString(report.CollectedAt) {
		t.Fatalf("unexpected collected_at format: %q", report.CollectedAt)
	}
}

func TestParseIPResponse(t *testing.T) {
	ipv6 := fmt.Sprintf("%x:%x::%x", 0x2001, 0xdb8, 0x10)
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "json", body: fmt.Sprintf(`{"ip":%q}`, net.IPv4(203, 0, 113, 10).String()), want: net.IPv4(203, 0, 113, 10).String()},
		{name: "plain", body: ipv6 + "\n", want: ipv6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseIPResponse([]byte(test.body))
			if err != nil {
				t.Fatalf("parseIPResponse() error = %v", err)
			}
			if got.String() != test.want {
				t.Fatalf("parseIPResponse() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseIPResponseRejectsInvalidData(t *testing.T) {
	if _, err := parseIPResponse([]byte(`{"ip":"not-an-ip"}`)); err == nil {
		t.Fatal("parseIPResponse() accepted invalid data")
	}
}

func TestParseIPProfileResponse(t *testing.T) {
	address := net.IPv4(203, 0, 113, 10).String()
	body := []byte(fmt.Sprintf(`{"success":true,"ip":%q,"type":"IPv4","country":"Exampleland","region":"Example Region","city":"Example City","connection":{"asn":64500,"org":"Example Hosting","isp":"Example ISP","domain":"example.test"},"timezone":{"id":"UTC"}}`, address))
	profile, err := parseIPProfileResponse(body, false)
	if err != nil {
		t.Fatalf("parseIPProfileResponse() error = %v", err)
	}
	if profile.Address != "" {
		t.Fatal("profile parser exposed hidden address")
	}
	if profile.ASN != "AS64500" || profile.Classification != "hosting" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if profile.Risk == nil || profile.Risk.Method != "heuristic" || profile.Risk.Score != 55 {
		t.Fatalf("unexpected risk: %+v", profile.Risk)
	}
}

func TestParseIPProfileResponseRejectsProviderError(t *testing.T) {
	if _, err := parseIPProfileResponse([]byte(`{"success":false,"message":"invalid"}`), false); err == nil {
		t.Fatal("profile parser accepted provider error")
	}
}

func TestInspectInterfacesDoesNotExposeAddresses(t *testing.T) {
	network := inspectInterfaces()
	if network.Interfaces < 0 || network.Up < 0 {
		t.Fatalf("invalid interface counts: %+v", network)
	}
	for _, iface := range (Report{Privacy: Privacy{Destinations: []string{
		"api4.ipify.org", "api6.ipify.org", "example.com", "www.cloudflare.com",
	}}}).Privacy.Destinations {
		if net.ParseIP(iface) != nil {
			t.Fatalf("destination leaked IP address: %s", iface)
		}
	}
}

func TestOfflineReportHasNoOutboundDestinations(t *testing.T) {
	report := Run(Config{Network: false, PublicIP: true, Intelligence: true})
	if len(report.Privacy.Destinations) != 0 {
		t.Fatalf("offline report listed destinations: %+v", report.Privacy.Destinations)
	}
	if len(report.DNS) != 0 || len(report.HTTPS) != 0 || report.IPProfile != nil {
		t.Fatal("offline report ran outbound probes")
	}
}
