# netdebug

Privacy-first network diagnostics CLI. Go. Zero runtime dependencies. Linux first, Windows/macOS supported.

`netdebug` provides a compact alternative to:

```bash
bash <(curl -Ls IP.Check.Place) -y
```

It checks local interface counts, public IPv4/IPv6 reachability, DNS resolution, and HTTPS connectivity. Output supports polished console, JSON, and Markdown formats.

## Install

```bash
go install github.com/neko233-com/netdebug/cmd/netdebug@latest
```

Or download a release binary for Linux, Windows, or macOS.

## Usage

```bash
# Console report; -y is accepted for compatibility and installs nothing.
netdebug -y

# Machine-readable output.
netdebug --format json

# Markdown report.
netdebug --format markdown > report.md

# IPv4 only, explicitly show detected public IPv4.
netdebug -4 --show-ip

# Avoid public IP probes entirely.
netdebug --no-public-ip

# Local-only mode: no outbound network requests.
netdebug --offline
```

## Privacy model

- No telemetry.
- No report links.
- No account, proxy credential, hostname, interface address, or local address sent in request payloads.
- Public IP is hidden in console/JSON/Markdown by default; `--show-ip` is explicit.
- Public IP probe services necessarily observe the transport source IP. Use `--no-public-ip` to skip those probes.
- `--offline` skips every outbound probe and inspects only local interface counts.
- Only fixed endpoints are contacted: `api4.ipify.org`, `api6.ipify.org`, `example.com`, `www.cloudflare.com`, and DNS lookups for `example.com` / `cloudflare.com`.
- No geo-IP, blacklist, media-unlock, or risk-scoring provider is queried.

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
