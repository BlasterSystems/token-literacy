# ADR-0002: Additive token model with OTLP and rollout taps

- **Status:** Accepted
- **Date:** 2026-11-09
- **Decision owners:** Bruno Pires
- **Scope:** Token semantics, Claude Code and Codex ingestion, dedupe, cost labelling
- **Tags:** tokens, cache, opentelemetry, codex, claude-code, data-model
- **Supersedes:** None
- **Superseded by:** None

## Context

Claude Code and Codex report the same concepts with different arithmetic:

- Claude Code: `input_tokens` excludes cache; `cache_read_tokens` and `cache_creation_tokens` are separate.
- Codex: `input_tokens` includes `cached_input_tokens`; `total_tokens = input_tokens + output_tokens`; adding the cached figure again doubles the total.

Claude Code exposes per-request `claude_code.llm_request` spans and token metrics. Codex writes per-response `token_usage_record` entries to rollout JSONL. A proxy would only see API-key traffic and would miss Claude Max / ChatGPT subscription OAuth, which is the population the article targets.

## Decision drivers

- Measure subscription traffic without a proxy or vendor API key.
- Per-turn fidelity, not daily aggregates.
- One additive row shape that supports cache hit rate, fresh share, context tax, and cache ROI.
- Never store prompt, tool, or message content.
- Survive replay: both harnesses rewrite records already written.

## Considered options

### Option 1: Proxy-only

A LiteLLM or custom gateway in front of every request. Uniform schema, but subscription OAuth bypasses it, and it makes the collector a critical path for inference.

### Option 2: Transcript-only

Parse `~/.claude/projects/**/*.jsonl` and Codex rollouts. No live path, and Claude transcripts replay aggressively: dedupe by `uuid` still overcounts because one reply appears under several `uuid`s sharing one `message.id`.

### Option 3: Harness-native taps

Claude Code OTel spans/metrics plus Codex rollout JSONL. Live and authoritative, but two parsers and explicit normalization.

## Decision

Use **harness-native taps**: Claude Code OTLP/HTTP JSON (`claude_code.llm_request` spans primary; `claude_code.token.usage` metrics fallback) and Codex rollout JSONL (`token_usage_record` primary; `event_msg token_count` only when no record exists). Normalize every request into an **additive** row:

`U = uncached input`, `R = cache read`, `W = cache write`, `O = output`, `P = U + R + W`, `total = P + O`.

Dedupe by Claude `request_id` and Codex `response_id`. Never add `input_tokens` and `cached_input_tokens` for Codex. Label subscription cost as API-equivalent; it is not billed spend.

## Consequences

### Positive

- Works on flat-rate subscriptions without intercepting traffic.
- Per-turn cache economics are visible; the same row shape works for both harnesses.
- No prompt content is required; only usage metadata crosses the parser.

### Negative

- Claude Code per-turn rows depend on beta tracing (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`); without it only aggregate metrics are available.
- Codex rollout schema can drift; the parser must fail visibly rather than guess.
- Two harnesses means two sets of replay/dedupe rules.

### Risks and mitigations

- **Double counting:** enforce `U + R == input_tokens` for Codex and `P == input + cache_read + cache_creation` for Claude; reject rows that fail.
- **Replay:** global `response_id` dedupe for Codex; `request_id` primary key for Claude.
- **Metric temporality:** use spans for per-turn rows; metrics are reconciliation/fallback only.
- **Content leakage:** parsers allow-list usage fields and discard prompts, tool output, and message bodies before insert.

## Implementation and validation

- Golden fixtures for Claude spans, Claude metrics, Codex record + event duplicate, resumed rollout, subagent rollout.
- Invariant tests reject negative, zero-total, and inconsistent rows.
- Reconcile one day of Codex history against `ccusage` within 1%; explain every delta.
- Manual smoke: one live turn per harness on macOS and Windows.

## Revisit triggers

- Claude Code removes per-request spans or changes the token attribute names.
- Codex adds a stable request id to OTel metrics, making rollout parsing unnecessary.
- A third harness is added; the additive model must remain the contract.

## References

- `docs/PLAN.md` M1, M2, Data contract
- ADR-0001 (Go collector)
- Claude Code monitoring: `https://code.claude.com/docs/en/monitoring-usage`
- Codex configuration reference: `https://developers.openai.com/codex/config-reference`
