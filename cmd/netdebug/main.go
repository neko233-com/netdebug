package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/netdebug/internal/diagnostics"
	"github.com/neko233-com/netdebug/internal/output"
	"github.com/neko233-com/netdebug/internal/updater"
)

var version = "0.1.3"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update" {
		runUpdate(os.Args[2:])
		return
	}
	var (
		format       string
		family       string
		language     string
		english      bool
		family4      bool
		family6      bool
		showIP       bool
		noPublicIP   bool
		offline      bool
		intelligence bool
		noColor      bool
		jsonOutput   bool
		markdownOut  bool
		compatYes    bool
		timeout      time.Duration
		showVersion  bool
	)

	flag.StringVar(&format, "format", "console", "output format: console, json, markdown")
	flag.StringVar(&family, "family", "all", "IP family: all, 4, 6")
	flag.StringVar(&language, "language", "cn", "output language: cn, en")
	flag.StringVar(&language, "l", "cn", "shorthand for --language")
	flag.BoolVar(&english, "E", false, "shorthand for --language en")
	flag.BoolVar(&family4, "4", false, "shorthand for --family 4")
	flag.BoolVar(&family6, "6", false, "shorthand for --family 6")
	flag.BoolVar(&showIP, "show-ip", false, "show public IP addresses; hidden by default")
	flag.BoolVar(&noPublicIP, "no-public-ip", false, "skip public IP probes")
	flag.BoolVar(&offline, "offline", false, "local-only mode; skip all outbound probes")
	flag.BoolVar(&intelligence, "intelligence", false, "query optional IP ASN/location/type intelligence")
	flag.BoolVar(&noColor, "no-color", false, "disable ANSI colors in console output")
	flag.BoolVar(&jsonOutput, "j", false, "shorthand for --format json")
	flag.BoolVar(&markdownOut, "m", false, "shorthand for --format markdown")
	flag.BoolVar(&compatYes, "y", false, "compatibility flag; netdebug installs no dependencies")
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "overall probe timeout")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Usage = usage
	flag.Parse()

	if showVersion {
		fmt.Printf("netdebug %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}
	_ = compatYes // Kept for command-line compatibility with the reference script.

	if jsonOutput {
		format = "json"
	}
	if markdownOut {
		format = "markdown"
	}
	if english {
		language = "en"
	}
	if family4 && family6 {
		fail("use only one of -4 or -6")
	}
	if family4 {
		family = "4"
	}
	if family6 {
		family = "6"
	}
	format = strings.ToLower(strings.TrimSpace(format))
	language = strings.ToLower(strings.TrimSpace(language))
	if format != "console" && format != "json" && format != "markdown" {
		fail("invalid --format %q; use console, json, or markdown", format)
	}
	if family != "all" && family != "4" && family != "6" {
		fail("invalid --family %q; use all, 4, or 6", family)
	}
	if language != "cn" && language != "en" {
		fail("invalid --language %q; use cn or en", language)
	}
	if timeout <= 0 {
		fail("--timeout must be greater than zero")
	}
	if intelligence && (noPublicIP || offline) {
		fail("--intelligence requires outbound public-IP probing; remove --no-public-ip/--offline")
	}

	report := diagnostics.Run(diagnostics.Config{
		Family:       family,
		Language:     language,
		ShowIP:       showIP,
		PublicIP:     !noPublicIP,
		Network:      !offline,
		Intelligence: intelligence,
		Timeout:      timeout,
		ToolVersion:  version,
	})

	color := format == "console" && !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
	if err := output.Render(os.Stdout, report, output.Options{Format: format, Color: color}); err != nil {
		fail("render output: %v", err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `netdebug %s — privacy-first network diagnostics

Usage:
  netdebug [flags]
  netdebug update

Examples:
  netdebug -y
  netdebug --language en
  netdebug --format json
  netdebug --format markdown > report.md
  netdebug -4 --show-ip

Flags:
`, version)
	flag.PrintDefaults()
}

func runUpdate(args []string) {
	if len(args) > 0 {
		fail("update does not accept positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := updater.Run(ctx, updater.Options{Repo: "neko233-com/netdebug", CurrentVersion: version})
	if err != nil {
		fail("update: %v", err)
	}
	if !result.Updated {
		fmt.Printf("netdebug %s already latest (%s)\n", result.CurrentVersion, result.LatestVersion)
		fmt.Printf("update route: %s\n", result.Route)
		return
	}
	fmt.Printf("netdebug updated: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Printf("update route: %s\n", result.Route)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "netdebug: "+format+"\n", args...)
	os.Exit(2)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
