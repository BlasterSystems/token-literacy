# Security Policy

## Reporting a vulnerability

Please do **not** report security vulnerabilities through public GitHub issues, discussions, or pull requests.

Use GitHub's **private vulnerability reporting** for this repository: open the **Security** tab → **Report a vulnerability**. This reaches the maintainers privately.

Include as much of the following as you can:

- `tokenledger version`, operating system, and install source (release tag or git revision).
- The affected component: OTLP ingest, Codex parser, SQLite ledger, config writer, dashboard, or Tauri shell.
- A minimal reproduction with redacted payloads. Never include subscription tokens, API keys, or prompt content.
- The impact you believe is possible: local file read/write, config corruption, token leakage, remote code execution, or denial of service.

## Supported versions

Only the latest tagged release receives security fixes. The `0.x` series ships new functionality in minor releases; security fixes may land as patch releases.

## Scope

In scope:

- The `tokenledger` binary, its HTTP server, parsers, SQLite handling, and configuration writers.
- The optional Tauri shell and its sidecar lifecycle.
- Release artifacts and checksums.

Out of scope:

- Claude Code, Codex, or vendor subscription behavior.
- The accuracy of provider-reported token counts or pricing.
- A compromised local user account, malicious `~/.claude/settings.json` or `~/.codex` controlled by an attacker who already has the user's privileges.
- Reports that require exposing the loopback server to a network.

## Security posture

- The HTTP server binds `127.0.0.1` only and rejects non-loopback `Host`/`RemoteAddr`.
- No authentication in v0.1; the trust boundary is the local user account.
- No outbound network calls and no telemetry.
- The ledger stores token counts and metadata only; prompts, tool output, and message bodies are discarded before insert.
- Configuration writes are backed up and never touch managed settings or project-local Claude Code settings.

## Response

This is a volunteer OSS project with no SLA. Maintainers will acknowledge a private report as soon as practical, investigate, and coordinate disclosure after a fix is available.
