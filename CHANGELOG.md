# Changelog

All notable changes to token-literacy are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Model pricing panel**: search the public OpenRouter catalog (no key) from the dashboard; picking a model caches its rates in SQLite (`model_rates`, schema v3) under the vendor-less model name so harness ids prefix-match, and editable fields let you correct or set rates by hand. Rate changes apply to the next turn without a restart; `~/.tokenledger/pricing.json` overrides still win on the same model prefix.
- **Default API price**: models with no known list price are costed at Claude Opus 5.5 list rates ($4 / $0.20 / $5 / $20 per 1M in/read/write/out) and marked `cost_source=default_price` (`≈` prefix in the dashboard cost column) instead of `none`.
- **DS v1.1 dashboard**: the dashboard now uses the BlasterSystems Design System v1.1 (oklch identity tokens, Fira Sans + DM Mono embedded — still no remote assets), with the product lockup, StatCards with computed InfoTips, ledger-line filters, width-aware charts (stacked bars with thread-outlined output, hit-rate line with peak annotation), and a system-plus-toggle theme remembered in localStorage.

### Fixed

- Charts re-render when the window is resized and chart panels contain their overflow, so a narrowed window no longer forces page-level horizontal scrolling that clipped both edges.

### Changed

- Price lookup resolves the longest matching override prefix deterministically; previously several matching prefixes resolved by map order.
- The tray shell opens the dashboard automatically once the collector is healthy on first launch, and the dashboard window is titled "TokL - Token Literacy".

## [0.1.0] — 2026-11-10

First public release: a local-first token ledger for Claude Code and Codex with per-turn fresh vs cached input, output, and API-equivalent cost.

### Added

- **Collector** (`tokenledger up`): loopback-only HTTP server on `127.0.0.1:4318`; OTLP/HTTP JSON ingest at `/v1/traces`, `/v1/metrics`, `/v1/logs` (gzip accepted); malformed payloads rejected without killing the process.
- **Claude Code adapter**: `claude_code.llm_request` span mapping (per-request `U/R/W/O` with `session.id`, `request_id`, `duration_ms`, `ttft_ms`, `stop_reason`, `attempt`); `claude_code.token.usage` metric fallback with cumulative-series handling and an aggregate-only banner.
- **Codex adapter**: rollout JSONL tailing from `$CODEX_HOME/sessions` or `~/.codex/sessions` with per-file `(path, inode, offset)` cursors, backfill, global `response_id` dedupe across resumed sessions, subagent attribution via `parent_thread_id`, and `event_msg token_count` fallback for record-free files. `input_tokens` already includes `cached_input_tokens`; the ledger never adds them twice.
- **Ledger**: SQLite (WAL) at `~/.tokenledger/ledger.db`, `calls`/`sessions`/`ingest_cursor`/`meta` schema, invariant rejection (no negative, zero-total, or non-additive rows), idempotent upsert.
- **Pricing**: embedded API list prices for current Anthropic and OpenAI models, `~/.tokenledger/pricing.json` override, unknown models stored with `cost_source=none`.
- **Dashboard** (`GET /`): summary cards (total tokens, cache hit rate, fresh share, context tax, API-equivalent cost, cost per output token), per-turn stacked `U/R/W/O` bars, cache-hit-rate line, harness/session/date filters, turn table. Inline SVG only; no external assets; light and dark themes.
- **Export**: `GET /export.csv` and `/export.json` with the dashboard's filters; `tokenledger export --csv|--json --since 7d --out FILE`. CSV numbers match the dashboard by construction (tested).
- **Setup**: `tokenledger setup [--dry-run] [--force] [--with-codex-otel]` merges the OTel env block into `~/.claude/settings.json` (and the Codex metrics block into `~/.codex/config.toml`) with backups and marker-key cleanup.
- **Doctor**: `tokenledger doctor` explains config, collector reachability, ledger integrity, last Claude span / Codex row, cursor lag, port, instance lock, WSL posture, and harness versions in one line per check.
- **Autostart**: `tokenledger install` / `uninstall [--purge] [--keep-config]` — per-user launchd agent on macOS, `HKCU\...\Run` entry on Windows; never a system service.
- **Single instance**: a second `up` opens the existing dashboard and exits; a stale pid lock is taken over.
- **Simulate**: `tokenledger simulate --fixture DIR` loads OTLP/rollout fixtures through the production ingest path.
- **Shell (optional)**: Tauri v2 menu-bar/tray app (`src-tauri/`) bundling the same Go binary as a sidecar — health-polled, restarted on crash, stopped on Quit. macOS runs as an accessory (no Dock icon). Unsigned in v0.1 (see README).
- **Distribution**: `scripts/build-release.sh` produces `tokenledger_<version>_<os>_<arch>` archives plus `checksums.txt` for `darwin/{amd64,arm64}` and `windows/{amd64,arm64}` (a macOS universal binary is a local convenience of `make release`); CI builds on Ubuntu, macOS, and Windows with a 75% coverage gate.

### Notes

- Cost figures are **API-equivalent list prices**, not billed subscription spend; the dashboard labels them accordingly.
- The collector stores usage metadata only — never prompts, tool output, or message bodies — and binds to loopback only.
- Claude Code per-turn rows require beta tracing (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`, written by `tokenledger setup`); without it, data is aggregate-only.

[0.1.0]: https://github.com/BlasterSystems/token-literacy/releases/tag/v0.1.0
