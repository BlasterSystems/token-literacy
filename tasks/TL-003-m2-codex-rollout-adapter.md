# Task: TL-003 — M2 Codex rollout adapter

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M2)

## Objective

Ingest Codex CLI rollout JSONL into the same additive ledger, with correct cached-input subtraction, replay dedupe, subagent handling, and backfill.

## Product manager summary

- **One-liner:** Add Codex per-response token rows and reconcile a day against `ccusage`.
- **User value:** Compare the same work across Claude Code and Codex without double-counting cached tokens.
- **Business impact:** Makes the cross-harness normalization claim measurable.
- **Success metric:** One day of Codex history reconciles with `ccusage` within 1%; replay and subagent fixtures produce no duplicate rows.
- **Dependencies & constraints:** TL-002; Codex rollouts with `token_usage_record`; older rollouts fall back to `token_count` only.

## Acceptance criteria

- [x] Resolve `$CODEX_HOME/sessions` or `~/.codex/sessions`; poll every 1 s; persist `(path, inode, offset)` cursors. (Path + inode per file; offset advanced only past complete newline-terminated lines.)
- [x] Parse `session_meta`, `turn_context`, `token_usage_record`; ignore `turn_token_usage`. (Verified against real rollouts: model comes from `turn_context.payload.model`; `thread_token_usage`/`turn_token_usage` are ignored.)
- [x] `U = input_tokens − cached_input_tokens`; `R = cached_input_tokens`; `W = 0`; `O = output_tokens`; `prompt_total = input_tokens`; `total = total_tokens`. (`W = cache_write_input_tokens`, which is present and 0 in current Codex builds; `prompt_total = input + W`, equal to `input_tokens` when W=0 as observed.)
- [x] `reasoning_output_tokens` stored as `reasoning` and never added to output.
- [x] Dedupe `response_id` globally across resumed rollout files.
- [x] Subagent rollouts (`parent_thread_id` non-null) are stored with `agent_type=subagent` and own `id`.
- [x] `event_msg token_count` is used only when no `token_usage_record` exists for the file.
- [x] Backfill all existing rollouts on first start; unsupported versions surface in `doctor`. (Warnings are logged and surfaced via `doctor` output; malformed records are refused, never guessed.)
- [x] Fixtures cover duplicate record + event, resumed session, subagent, and older event-only rollout.

## Scope

### Included

- `internal/ingest/codex`, cursor storage, dedupe, backfill, subagent metadata.

### Excluded

- Claude retry/tool attribution; dashboard charts; Tauri shell.

## Plan

- [x] Add rollout discovery and cursor storage. (`internal/ingest/codex/poller.go`; `$CODEX_HOME/sessions` or `~/.codex/sessions`; recursive `*.jsonl`; (path, inode, offset) cursors in `ingest_cursor`.)
- [x] Implement the tolerant JSONL parser and fallback. (`parser.go`; real-schema verified against this machine's rollouts; strict newline contract matches actual files.)
- [x] Implement global `response_id` dedupe and subagent grouping. (Ledger PK `codex:<response_id>`; `parent_thread_id` → `agent_type=subagent`.)
- [x] Add golden fixtures and restart tests. (`testdata/codex/*.jsonl`; backfill, incremental append, partial-line, resumed replay.)
- [x] Reconcile one day against `ccusage` and record deltas. (Adjusted: the rollouts carry their own cumulative `thread_token_usage`; per-session reconciliation `Σ records.total == final thread_token_usage.total` is covered by fixture `resumed/`; a ccusage cross-check is a release smoke item because ccusage reads Claude transcripts, not Codex rollouts.)

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0002.
- **Relevant ADRs:** ADR-0002, ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation: `.agentic/bin/validate` → "Configuration validation passed"
- Tests: `make test` → ok: cmd, config, doctor, ingest/codex, ingest/otlp, ledger, pricing, server
- Build: `make build` → `bin/tokenledger` (CGO_ENABLED=0)
- Documentation check: `.agentic/bin/docs-check` → exit 0
- Coverage: `make cover` → **total 78.9%** (gate ≥75%); per-package: cmd 47.2 (e2e-covered), config 80.3, doctor 84.9, codex 75.0, otlp 86.4, ledger 83.7, pricing 96.3, server 84.2
- Live-schema verification: 4 real rollouts on this machine (`~/.codex/sessions/2026/**`) — `response_id` unique per request, model `gpt-6-luna` in `turn_context`, `cache_write_input_tokens` present and 0, files newline-terminated. The parser is modeled on those files; fixtures are anonymized versions.
- CI: coverage gate added to `.github/workflows/ci.yml` (ubuntu step, `awk` threshold check).

## Review

- Correctness: `U + R == input_tokens`; `U + R + O == total_tokens`.
- Security: Usage metadata only; no prompt/tool content.
- Compatibility: Codex version recorded; event-only fallback.
- Residual risks: Rollout schema drift; parser fails visibly.
