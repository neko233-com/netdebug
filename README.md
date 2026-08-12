# netdebug

Privacy-first network diagnostics CLI. Go. Zero runtime dependencies. Linux first, Windows/macOS supported.

`netdebug` provides a compact alternative to:

```bash
bash <(curl -Ls IP.Check.Place) -y
```

It checks local interface counts, public IPv4/IPv6 reachability, DNS resolution, and HTTPS connectivity. Output supports a reference-style console dashboard with sections, status badges, latency tables, and risk bars, plus JSON and Markdown formats in Chinese or English.

## Install

```bash
go install github.com/neko233-com/netdebug/cmd/netdebug@latest

# Linux/macOS: HTTPS download, mirror speed probe, SHA-256 verification.
curl -fsSL https://raw.githubusercontent.com/neko233-com/netdebug/main/install.sh | sh
```

PowerShell:

```powershell
irm https://raw.githubusercontent.com/neko233-com/netdebug/main/install.ps1 | iex
```

One-line install and report, equivalent to the reference command:

```bash
curl -fsSL https://raw.githubusercontent.com/neko233-com/netdebug/main/install.sh | sh -s -- --run
# -y is also accepted: curl -fsSL https://raw.githubusercontent.com/neko233-com/netdebug/main/install.sh | sh -s -- -y
```

PowerShell:

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/neko233-com/netdebug/main/install.ps1))) -Run
```

After installation, simply running `netdebug` prints the same console report.

Installers are idempotent. They install release binaries, verify `checksums.txt`, register `NETDEBUG_HOME`, and update PATH for future shells. Set `NETDEBUG_DIRECT_ONLY=1` to disable mirrors, or `NETDEBUG_UPDATE_MIRRORS` to provide comma-separated mirror prefixes.

## Update

```bash
netdebug update
```

Update probes official GitHub and configured public mirror routes, selects the fastest reachable route, downloads the archive and checksum through that route, then verifies SHA-256 before replacement. Use `NETDEBUG_DIRECT_ONLY=1 netdebug update` to force official GitHub only. A mirror can improve reachability but is not an authenticity authority; checksum verification remains mandatory.

## Usage

```bash
# Console report; -y is accepted for compatibility and installs nothing.
netdebug -y

# Chinese is the default; English output.
netdebug --language en
netdebug -l en
netdebug -E

# Machine-readable output.
netdebug --format json

# Markdown report.
netdebug --format markdown > report.md

# IPv4 only; console prints the detected public IPv4 by default.
netdebug -4

# Show IP in JSON/Markdown too; hide it everywhere when needed.
netdebug --format json --show-ip
netdebug --hide-ip

# Opt-in IP profile: ASN, organization, coarse location, network type,
# provider flags when available, and clearly-labelled heuristic risk.
netdebug --intelligence

# Avoid public IP probes entirely.
netdebug --no-public-ip

# Local-only mode: no outbound network requests.
netdebug --offline
```

`--language cn` is the default. JSON keeps stable English field names and status enums; `language` identifies the display locale. `collected_at` uses UTC format `yyyy-MM-dd HH:mm:ss,SSS`.

## Scope comparison

`IP.Check.Place` / `IPQuality` is a broader, provider-heavy audit. `netdebug` is a smaller privacy-first baseline with structured output and no runtime dependencies.

| Area | Reference script | netdebug |
|---|---|---|
| Local network, DNS, HTTPS | Yes | Yes |
| Multi-database IP cross-check | Many providers | Optional `ipwho.is` profile |
| Streaming / AI unlock | Yes | Not yet |
| Port 25 / DNSBL | Yes | Not yet |
| Output | Rich terminal, JSON options | Console, JSON, Markdown; `cn` / `en` |
| Dependencies | Installs shell tools with `-y` | None at runtime |
| Privacy posture | Includes telemetry/report-link paths | No telemetry, upload, or report links |

Observed comparison on the authorized test host: current reference script (`v2026-08-09`) took about 50 seconds and produced about 560 lines / 114 KB, then exited with code 1. It provides broader coverage, but has more external dependencies and provider failure points. `netdebug` favors bounded probes, stable machine output, and explicit opt-in intelligence.

## Privacy model

- No telemetry.
- No report links.
- No account, proxy credential, hostname, interface address, or local address sent in request payloads.
- Environment proxy settings are not inherited, so proxy credentials are not forwarded.
- Console prints the detected public IP locally by default, matching the reference report style; it is never uploaded. JSON/Markdown hide it by default. Use `--show-ip` for every format or `--hide-ip` everywhere.
- Public IP probe services necessarily observe the transport source IP. Use `--no-public-ip` to skip those probes.
- `--offline` skips every outbound probe and inspects only local interface counts.
- `--intelligence` is opt-in and contacts [`ipwho.is`](https://ipwhois.io/documentation) for ASN, organization, coarse location, and network metadata. Its free response may not include threat flags.
- Risk score is heuristic unless provider security flags are present; it is not a blacklist or fraud verdict.
- Only fixed endpoints are contacted: `api4.ipify.org`, `api6.ipify.org`, `example.com`, `www.cloudflare.com`, and DNS lookups for `example.com` / `cloudflare.com`; `--intelligence` additionally contacts `ipwho.is`.
- Without `--intelligence`, no geo-IP, blacklist, media-unlock, or risk-scoring provider is queried.

Network access remains visible to the endpoints listed above. Review or disable probes before using the command in a restricted environment.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/netdebug
```

Build matrix targets:

```text
linux/amd64  linux/arm64  windows/amd64  darwin/amd64  darwin/arm64
```

## License

MIT. See [LICENSE](LICENSE).
