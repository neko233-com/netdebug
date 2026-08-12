# Security Policy

## Reporting

Do not include real IP addresses, hostnames, usernames, tokens, proxy URLs, or account details in issues.

Report security problems privately through GitHub Security Advisories when available. If unavailable, open an issue containing only a minimal, sanitized reproduction and request private contact.

## Privacy boundary

`netdebug` does not upload reports or telemetry. Public IP probes still expose the transport source IP to the selected probe service, as required to discover it. Use `--no-public-ip` when that is unacceptable.
