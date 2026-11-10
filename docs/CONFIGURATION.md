# Configuration

tokenledger is configured almost entirely through the harness files it writes and one optional override file. No server config file exists; the binary is configured by flags.

## Collector (`tokenledger up`)

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--port` | `TOKENLEDGER_ADDR` (as `127.0.0.1:<port>`) | `4318` | loopback bind port |
| `--db` | `TOKENLEDGER_DB` | `~/.tokenledger/ledger.db` | ledger path (created, WAL) |
| `--codex-dir` | `CODEX_HOME` (then `~/.codex/sessions`) | — | Codex rollout directory |
| `--no-codex` | — | off | disable the rollout poller |
| `--no-open` | — | off | do not open the dashboard in a browser |

A second `up` detects the live pid lock at `~/.tokenledger/tokenledger.lock`, opens the existing dashboard, and exits.

## Claude Code (`tokenledger setup`)

Merges an `env` block into `~/.claude/settings.json` (honors `CLAUDE_CONFIG_DIR`), preserving all other content. `--dry-run` prints the diff; `--force` prunes stale marker keys; a `.bak-tokenledger` backup is written before any change; unparsable files are refused, never overwritten.

Keys written (marker prefixes `OTEL_EXPORTER_OTLP*`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER`, `OTEL_TRACES_EXPORTER*`, `OTEL_METRIC_EXPORT_INTERVAL`, `CLAUDE_CODE_ENABLE_TELEMETRY`, `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA`):

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "none",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:4318",
    "OTEL_METRIC_EXPORT_INTERVAL": "5000",
    "CLAUDE_CODE_ENHANCED_TELEMETRY_BETA": "1",
    "OTEL_TRACES_EXPORTER": "otlp",
    "OTEL_TRACES_EXPORT_INTERVAL": "2000"
  }
}
```

`--endpoint URL` retargets the OTLP endpoint. `tokenledger uninstall` removes these keys (backup written).

## Codex (`tokenledger setup --with-codex-otel`)

Appends (or replaces) a marked block in `~/.codex/config.toml` (honors `CODEX_HOME`):

```toml
# BEGIN tokenledger
[otel.metrics_exporter.otlp-http]
endpoint = "http://127.0.0.1:4318"
protocol = "http/json"
# END tokenledger
```

Token counts still come from rollout files; this block only enables Codex's own cost metrics.

## Pricing override (`~/.tokenledger/pricing.json`)

Optional. Maps a model-id prefix to USD-per-million-token rates; overrides win over the embedded table; unknown models are priced at the Claude Opus 5.5 default (`cost_source=default_price`). Cached OpenRouter rates and manual edits live in the ledger's `model_rates` table and layer beneath this file.

```json
{
  "claude-sonnet-4-5": { "input": 3, "cache_read": 0.3, "cache_write": 3.75, "output": 15 }
}
```

`cache_write` is the effective 5-minute prompt-cache premium (1.25× input on Anthropic list prices); OpenAI models omit it (no write billing).

## Autostart

`tokenledger install` writes a per-user login item — `~/Library/LaunchAgents/com.blastersystems.tokenledger.plist` on macOS (loaded with `launchctl`), `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\com.blastersystems.tokenledger` on Windows. Never a system service, no admin rights. `tokenledger uninstall` removes it; `--purge` also deletes `~/.tokenledger` (ledger included); `--keep-config` leaves harness markers untouched.
