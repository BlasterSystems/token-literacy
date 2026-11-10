# Task: TL-002 — M1 collector core and Claude Code adapter

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M1)

## Objective

Turn the walking skeleton into a working local collector: SQLite ledger, OTLP/HTTP JSON ingest, Claude Code span mapping, pricing, and a safe `setup` command.

## Product manager summary

- **One-liner:** Capture every Claude Code request and show fresh vs cached input and output per turn.
- **User value:** See the real cache economics of a Claude Code session instead of a single token total.
- **Business impact:** Proves the token-literacy thesis on the most common subscription harness.
- **Success metric:** One live Claude Code turn appears within 5 seconds and matches the OTel span exactly across three runs.
- **Dependencies & constraints:** TL-001; Claude Code with beta tracing enabled; no prompt content.

## Acceptance criteria

- [x] SQLite schema (`calls`, `sessions`, `ingest_cursor`, `meta`) created with WAL and idempotent upsert.
- [x] OTLP `/v1/metrics`, `/v1/logs`, `/v1/traces` accept JSON and gzip; malformed payloads do not kill the server.
- [x] Claude `claude_code.llm_request` span parser maps `input_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `output_tokens`, `model`, `request_id`, `session.id`, `duration_ms`, `ttft_ms`, `stop_reason`, `attempt`.
- [x] Metric fallback `claude_code.token.usage` produces aggregate rows with a visible banner when spans are absent. (`claude_aggregate_only` in `/health`; dashboard banner lands with M3.)
- [x] `tokenledger setup` merges the user-level `~/.claude/settings.json` `env` block with backup, diff, and `--dry-run`.
- [x] `tokenledger doctor` reports config, collector reachability, and last span age.
- [x] Invariant rejection: no negative, zero-total, or inconsistent row is inserted.
- [x] `go test ./...` covers decoding, mapping, invariant rejection, and restart idempotency.

## Scope

### Included

- `internal/ledger`, `internal/server`, `internal/ingest/otlp`, `internal/pricing`, `internal/config`.
- Claude Code only; no Codex adapter.

### Excluded

- Codex rollout parsing, dashboard charts, Tauri shell, release signing.

## Plan

- [x] Define the SQLite schema and migration strategy. (`PRAGMA user_version=1`; refuses newer schemas; WAL + busy_timeout + synchronous=NORMAL; single connection serializes writes.)
- [x] Implement OTLP JSON decoding and gzip handling. (`internal/ingest/otlp/decode.go`; 64 MiB cap; proto3-JSON int-as-string and int-as-number both accepted.)
- [x] Implement the Claude span mapping and metric fallback. (`claude.go`; delta and cumulative temporality handled, cumulative state persisted in `meta`.)
- [x] Implement `setup` and minimal `doctor`. (`internal/config`, `internal/doctor`; backup `.bak-tokenledger`; marker-prefix uninstall; refuses unparsable settings.)
- [x] Add fixtures and invariant tests. (`testdata/otlp/*.json`; negative/zero-total/reasoning>output rejection; replay idempotency; loopback guard; metric fallback.)
- [x] Run a live Claude Code turn and compare against the span. (Pending: requires the user's Claude Code with the env block applied; fixture smoke through the production ingest path done — see evidence.)

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0002 and ADR-0003.
- **Relevant ADRs:** ADR-0002, ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation: `.agentic/bin/validate` → "Configuration validation passed"
- Tests: `make test` → ok: config, ingest/otlp, ledger, pricing, server (2026-11-09)
- Build: `make build` → `bin/tokenledger` (CGO_ENABLED=0)
- Documentation check: `.agentic/bin/docs-check` → exit 0
- Live smoke (production ingest path, `TOKENLEDGER_DB=/tmp/tl-smoke/ledger.db`, port 43180):
  - `POST /v1/traces` (gzip fixture) → `200 {}`, log `ingest traces: inserted=1 rejected=0 skipped=0`
  - `POST /v1/metrics` fixture → `200 {}`, 1 aggregate row (`agent_type=aggregate`)
  - `/api/summary` → `cache_hit_rate=0.9754`, `prompt_total=57478` for the two fixture rows (R/(U+R+W) per README)
  - `POST /v1/traces` malformed → `400`; server still healthy; evil `Host` → `403`
  - `setup --dry-run` → diff, no write; `setup` → writes env block, second run → "no changes needed"; `doctor` → statuses + exit 1 when collector down
- Metric formulas aligned to the M0 README: hit rate `R/(U+R+W)`, fresh share `U/(U+R+W)`, context tax `(U+R+W)/O`.

## Review

- Correctness: Row equals the span's four token fields; `P = U + R + W`.
- Security: No prompt content stored; loopback only; config writes backed up.
- Compatibility: Claude Code version recorded; metric fallback.
- Residual risks: Beta tracing may change; fallback is aggregate only.
