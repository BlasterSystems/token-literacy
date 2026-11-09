# Architecture

## Purpose

This document is the current high-level map of **token-literacy**. Keep it concise. Detailed historical rationale belongs in `docs/adr/`.

## System Context

- **Operator:** one developer on a macOS or Windows workstation. No application auth; the trust boundary is the local user account.
- **Measured harnesses (v1):** Claude Code (OTel/HTTP JSON) and OpenAI Codex CLI (rollout JSONL). Both are subscription-friendly: telemetry is client-side and independent of how inference is billed.
- **Explicitly out of scope (v1):** LiteLLM, OpenRouter, omp, OpenCode, Cursor, Copilot, cloud sync, multi-user mode, and any proxy in front of vendor OAuth traffic.
- **Trust boundary:** the collector binds `127.0.0.1` only. It reads two user-owned inputs (`~/.claude/settings.json`, `~/.codex/sessions`) and writes one user-owned database (`~/.tokenledger/ledger.db`). No prompt or tool content leaves the machine; none is stored.

## Components

| Component | Responsibility |
|---|---|
| `cmd/tokenledger` | CLI entry point: `setup`, `up`, `doctor`, `export`, `simulate`, `install`, `uninstall`, `version`. |
| `internal/server` | Loopback HTTP server: OTLP JSON ingest (`/v1/metrics`, `/v1/logs`, `/v1/traces`), dashboard, JSON API, CSV export, `/health`. |
| `internal/ingest/otlp` | OTLP JSON decoder and Claude Code span/metric mapping. |
| `internal/ingest/codex` | Rollout discovery, incremental tail, JSONL parser, replay/subagent dedupe. |
| `internal/ledger` | SQLite schema, idempotent upsert, queries, rollups. |
| `internal/pricing` | List-price table and API-equivalent cost calculation; unknown models are `cost_source=none`. |
| `internal/config` | Claude Code settings merge (JSON), Codex path discovery, backups and uninstall markers. |
| `ui/` | Server-rendered HTML and inline SVG; no framework, CDN, or build step. |
| `src-tauri/` (M5) | Optional Tauri v2 tray/menu-bar shell; spawns and supervises the Go binary, opens the dashboard, manages autostart. The Go binary remains complete without it. |
| `testdata/` | Redacted OTLP and rollout fixtures used by `go test` and `tokenledger simulate`. |

## Data and State

- **Authoritative store:** SQLite (WAL) at `~/.tokenledger/ledger.db`, per user.
- **`calls`:** one normalized row per model request. Additive fields `input_uncached`, `cache_read`, `cache_write`, `output`, `reasoning`; derived `prompt_total = U + R + W`, `total = prompt_total + output`. `cost_usd` is API-equivalent, never billed subscription spend.
- **`sessions`:** per harness/session/agent metadata (cwd, model, first/last seen, billing mode).
- **`ingest_cursor`:** `(path, inode, offset)` for Codex rollout files, so restarts do not duplicate or skip rows.
- **Dedupe:** Claude `request_id`; Codex `response_id` across all rollout files. Codex `token_usage_record` wins over `event_msg token_count`; never add both.
- **Retention:** raw per-request rows retained indefinitely at single-developer volume. No prompt text, tool output, or message bodies are persisted.
- **Derived metrics:** cache hit rate (`R/P`), fresh share (`U/P`), context tax (`P/O`), cache ROI, cost per output token, retry waste, cache-expiry miss.

## Integrations

- **Claude Code → collector:** OTLP/HTTP JSON. `claude_code.llm_request` spans are the per-request source; `claude_code.token.usage` and `claude_code.cost.usage` metrics are reconciliation/fallback. Setup writes the `env` block in `~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR`); repository `.claude/settings.json` OTel variables are ignored by Claude Code.
- **Codex → collector:** rollout files under `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl`, tailed incrementally. Optional `[otel]` config only for cost reconciliation; it is not required for token counts.
- **Pricing:** embedded list prices plus an optional `~/.tokenledger/pricing.json` override. Subscription rows carry `billing_mode=subscription` and `cost_source=list_price`.
- **No outbound network calls.** The collector never sends telemetry anywhere.

## Deployment and Operations

- **Distribution:** one static Go binary per OS/arch (`darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64`), `CGO_ENABLED=0`, no runtime dependency. Optional per-user installer and autostart (`launchd` on macOS, per-user `HKCU\...\Run` or Startup shortcut on Windows). Never a Windows service: it would read the wrong user profile.
- **Process model:** `tokenledger up` runs in the foreground for development; `tokenledger install` registers a per-user login item. Single-instance guard; a second invocation opens the existing dashboard.
- **Ports:** `127.0.0.1:4318` for OTLP and dashboard by default; configurable with `TOKENLEDGER_ADDR`.
- **Observability:** `/health` reports version, DB path, last Claude span age, last Codex row, and cursor lag. `doctor` checks config, harness versions, permission errors, and WSL endpoint hazards.
- **Recovery:** SQLite WAL plus idempotent upsert; deleting the database rebuilds history from Codex rollouts and future Claude spans. Claude Code history before install is not imported in v1.
- **Windows/WSL:** native Windows harnesses are supported first. A harness inside WSL cannot reach the Windows loopback collector; `doctor` detects WSL and prints the host-IP/firewall alternative.
- **Tauri shell (M5):** per-OS builds in CI; the Go binary is bundled as a sidecar and must be signed with the app; no Mac App Store distribution (sandbox conflicts with reading `~/.claude` / `~/.codex`).

## Agent-Guided Maintenance Boundary

When the agent-guided bootstrap workflow is used, repository evidence and agent proposals are untrusted. The deterministic core owns rendering, planning, apply, rollback, and validation. The agent stops after preview; a human separately confirms the recomputed plan in an interactive terminal. See `docs/AGENT_GUIDED.md`.

## Architecture Decision Index

Read `docs/adr/README.md` before individual decision records. Update this overview when an accepted ADR changes the current architecture.
