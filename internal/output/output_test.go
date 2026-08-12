package output

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/neko233-com/netdebug/internal/diagnostics"
)

func sampleReport() diagnostics.Report {
	address := net.IPv4(203, 0, 113, 10).String()
	return diagnostics.Report{
		Tool:       "netdebug",
		Platform:   diagnostics.Platform{OS: "linux", Arch: "amd64"},
		Privacy:    diagnostics.Privacy{Telemetry: false, ReportUpload: false},
		PublicIPv4: &diagnostics.IPProbe{Status: "pass", Address: address, Source: "api4.ipify.org"},
		Summary:    diagnostics.Summary{Passed: 1},
	}
}

func TestJSONRenderHidesAddressWhenNotCollected(t *testing.T) {
	report := sampleReport()
	address := report.PublicIPv4.Address
	report.PublicIPv4.Address = ""
	var output bytes.Buffer
	if err := Render(&output, report, Options{Format: "json"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), address) {
		t.Fatal("JSON output leaked hidden address")
	}
}

func TestMarkdownRenderHidesAddressByDefault(t *testing.T) {
	report := sampleReport()
	address := report.PublicIPv4.Address
	report.PublicIPv4.Address = ""
	var output bytes.Buffer
	if err := Render(&output, report, Options{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), address) {
		t.Fatal("Markdown output leaked hidden address")
	}
}

func TestConsoleRender(t *testing.T) {
	report := sampleReport()
	report.PublicIPv4.Address = ""
	var output bytes.Buffer
	renderConsole(&output, report, false)
	if !strings.Contains(output.String(), "netdebug") || !strings.Contains(output.String(), "available (hidden)") {
		t.Fatalf("unexpected console output: %s", output.String())
	}
}

func TestConsoleRenderShowsAddressWhenCollected(t *testing.T) {
	report := sampleReport()
	var output bytes.Buffer
	renderConsole(&output, report, false)
	if !strings.Contains(output.String(), report.PublicIPv4.Address) || !strings.Contains(output.String(), "local") {
		t.Fatalf("console output did not show collected address: %s", output.String())
	}
}

func TestBilingualRender(t *testing.T) {
	for _, test := range []struct {
		language string
		want     string
	}{
		{language: "cn", want: "隐私"},
		{language: "en", want: "Privacy"},
	} {
		report := sampleReport()
		report.Language = test.language
		var output bytes.Buffer
		renderConsole(&output, report, false)
		if !strings.Contains(output.String(), test.want) {
			t.Fatalf("language %s missing %q: %s", test.language, test.want, output.String())
		}
	}
}

func TestConsoleProfileRendersRiskBar(t *testing.T) {
	report := sampleReport()
	report.Language = "cn"
	report.IPProfile = &diagnostics.IPProfile{
		Status: "pass", Source: "ipwho.is", Classification: "hosting",
		Risk: &diagnostics.RiskAssessment{Score: 55, Level: "medium-high", Method: "heuristic"},
	}
	var output bytes.Buffer
	renderConsole(&output, report, false)
	if !strings.Contains(output.String(), "████") || !strings.Contains(output.String(), "中高风险") {
		t.Fatalf("console profile missing risk visualization: %s", output.String())
	}
}

func TestProfileOutputHidesAddress(t *testing.T) {
	report := sampleReport()
	address := report.PublicIPv4.Address
	report.PublicIPv4.Address = ""
	report.IPProfile = &diagnostics.IPProfile{
		Status: "pass", Source: "ipwho.is", Address: "", ASN: "AS64500",
		Organization: "Example Hosting", City: "Example City", Country: "Exampleland",
		Classification: "hosting", Risk: &diagnostics.RiskAssessment{Score: 55, Level: "medium-high", Method: "heuristic"},
	}
	var output bytes.Buffer
	if err := Render(&output, report, Options{Format: "json"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), address) {
		t.Fatal("profile output leaked hidden address")
	}
	if !strings.Contains(output.String(), "AS64500") || !strings.Contains(output.String(), "medium-high") {
		t.Fatalf("profile output missing fields: %s", output.String())
	}
}
