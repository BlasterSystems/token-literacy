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

- [ ] SQLite schema (`calls`, `sessions`, `ingest_cursor`, `meta`) created with WAL and idempotent upsert.
- [ ] OTLP `/v1/metrics`, `/v1/logs`, `/v1/traces` accept JSON and gzip; malformed payloads do not kill the server.
- [ ] Claude `claude_code.llm_request` span parser maps `input_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `output_tokens`, `model`, `request_id`, `session.id`, `duration_ms`, `ttft_ms`, `stop_reason`, `attempt`.
- [ ] Metric fallback `claude_code.token.usage` produces aggregate rows with a visible banner when spans are absent.
- [ ] `tokenledger setup` merges the user-level `~/.claude/settings.json` `env` block with backup, diff, and `--dry-run`.
- [ ] `tokenledger doctor` reports config, collector reachability, and last span age.
- [ ] Invariant rejection: no negative, zero-total, or inconsistent row is inserted.
- [ ] `go test ./...` covers decoding, mapping, invariant rejection, and restart idempotency.

## Scope

### Included

- `internal/ledger`, `internal/server`, `internal/ingest/otlp`, `internal/pricing`, `internal/config`.
- Claude Code only; no Codex adapter.

### Excluded

- Codex rollout parsing, dashboard charts, Tauri shell, release signing.

## Plan

- [ ] Define the SQLite schema and migration strategy.
- [ ] Implement OTLP JSON decoding and gzip handling.
- [ ] Implement the Claude span mapping and metric fallback.
- [ ] Implement `setup` and minimal `doctor`.
- [ ] Add fixtures and invariant tests.
- [ ] Run a live Claude Code turn and compare against the span.

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0002 and ADR-0003.
- **Relevant ADRs:** ADR-0002, ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: Row equals the span's four token fields; `P = U + R + W`.
- Security: No prompt content stored; loopback only; config writes backed up.
- Compatibility: Claude Code version recorded; metric fallback.
- Residual risks: Beta tracing may change; fallback is aggregate only.
