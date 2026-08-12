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
