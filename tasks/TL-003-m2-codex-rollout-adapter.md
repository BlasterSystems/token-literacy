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

- [ ] Resolve `$CODEX_HOME/sessions` or `~/.codex/sessions`; poll every 1 s; persist `(path, inode, offset)` cursors.
- [ ] Parse `session_meta`, `turn_context`, `token_usage_record`; ignore `turn_token_usage` running totals.
- [ ] `U = input_tokens − cached_input_tokens`; `R = cached_input_tokens`; `W = 0`; `O = output_tokens`; `prompt_total = input_tokens`; `total = total_tokens`.
- [ ] `reasoning_output_tokens` stored as `reasoning` and never added to output.
- [ ] Dedupe `response_id` globally across resumed rollout files.
- [ ] Subagent rollouts (`parent_thread_id` non-null) are stored with `agent_type=subagent` and own `id`.
- [ ] `event_msg token_count` is used only when no `token_usage_record` exists for the file.
- [ ] Backfill all existing rollouts on first start; unsupported versions surface in `doctor`.
- [ ] Fixtures cover duplicate record + event, resumed session, subagent, and older event-only rollout.

## Scope

### Included

- `internal/ingest/codex`, cursor storage, dedupe, backfill, subagent metadata.

### Excluded

- Claude retry/tool attribution; dashboard charts; Tauri shell.

## Plan

- [ ] Add rollout discovery and cursor storage.
- [ ] Implement the tolerant JSONL parser and fallback.
- [ ] Implement global `response_id` dedupe and subagent grouping.
- [ ] Add golden fixtures and restart tests.
- [ ] Reconcile one day against `ccusage` and record deltas.

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0002.
- **Relevant ADRs:** ADR-0002, ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: `U + R == input_tokens`; `U + R + O == total_tokens`.
- Security: Usage metadata only; no prompt/tool content.
- Compatibility: Codex version recorded; event-only fallback.
- Residual risks: Rollout schema drift; parser fails visibly.
