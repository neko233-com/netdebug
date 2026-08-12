package diagnostics

import (
	"fmt"
	"net"
	"testing"
)

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
