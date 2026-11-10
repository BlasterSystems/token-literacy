# HTTP API

The collector serves everything on `127.0.0.1` only (ADR-0003). Requests from non-loopback source addresses, or with a non-loopback `Host` header (DNS-rebinding defense), are rejected with `403`. No authentication in v0.1.

## Ingest (OTLP/HTTP JSON)

| Route | Signal | Notes |
|---|---|---|
| `POST /v1/traces` | spans | `claude_code.llm_request` spans become per-turn rows |
| `POST /v1/metrics` | gauge/sum | `claude_code.token.usage` becomes aggregate rows (`agent_type=aggregate`) |
| `POST /v1/logs` | logs | accepted and discarded; never stored |

- `Content-Encoding: gzip` accepted on all three; body limit 64 MiB.
- Responses are `200 {}` (OTLP success) or `400` for undecodable bodies. Rows that fail their harness invariant are logged and skipped, never inserted.
- Claude mapping: `input_tokens` → `input_uncached`, `cache_read_tokens` → `cache_read`, `cache_creation_tokens` → `cache_write`, `output_tokens` → `output`; `prompt_total = U+R+W`, `total_tokens = P+O`. Dedupe key: `claude-code:<request_id>`.
- Codex rows come from rollout files, not HTTP (see `docs/PLAN.md`); `cached_input_tokens` is a subset of `input_tokens` and is never added again.

## Queries

| Route | Parameters | Response |
|---|---|---|
| `GET /health` | — | `{status, version, uptime_seconds, db, calls_total, calls_by_harness, last_claude_span_ms, last_codex_row_ms, claude_aggregate_only}` |
| `GET /api/summary` | `since`, `harness`, `session` | `{calls, input_fresh, cache_read, cache_write, output, reasoning, prompt_total, total_tokens, cost_usd, costed_calls, cache_hit_rate, fresh_share, context_tax, cost_per_output_token, cost_estimate_usd, unpriced_calls, unpriced_used_default}` |
| `GET /api/rows` | `since`, `harness`, `session`, `limit`, `offset`, `order=asc\|desc` | array of call rows |
| `GET /api/sessions` | `since`, `harness` | array of `{harness, session_id, first_seen, last_seen, turns, total_tokens}` |
| `GET /api/rates` | — | `{rates: [{model, name, input, cache_read, cache_write, output, source, updated_at}], default: {model, price}}` — cached model rates in USD per million tokens (`source`: `openrouter` or `manual`) plus the default-price fallback info |
| `PUT /api/rates` | JSON body | Upserts one rate `{model, name, input, cache_read, cache_write, output, source}`; `model` and non-negative rates are required; unknown `source` defaults to `manual` |
| `DELETE /api/rates` | `model` | Removes one cached rate |

`cost_usd` sums only rows stored with a price; `cost_estimate_usd` is the dashboard's live estimate for rows without one (`unpriced_calls`), priced at current rates — `unpriced_used_default` reports whether the Claude Opus 5.5 fallback applied. Exports keep the raw per-row values.

`since` accepts epoch milliseconds, a relative duration (`7d`, `12h`), or RFC 3339.

Derived metrics (from the additive row, never `total_tokens`): `cache_hit_rate = R/(U+R+W)`, `fresh_share = U/(U+R+W)`, `context_tax = (U+R+W)/O`.

## Export

| Route | Notes |
|---|---|
| `GET /export.csv` | Same filters as the queries; ascending, unpaginated; `Content-Type: text/csv`; attachment `tokenledger-export.csv` |
| `GET /export.json` | Same rows as JSON; attachment `tokenledger-export.json` |

CSV columns (stable contract): `id, harness, session_id, agent_type, turn_index, request_id, model, billing_mode, input_uncached, cache_read, cache_write, output, reasoning, prompt_total, total_tokens, cost_usd, cost_source, started_at, ended_at, duration_ms, ttft_ms, stop_reason, attempt, source, harness_version`.

Errors on query routes are `{"error": "..."}` with status 500; a closed ledger answers `503 ledger unavailable`.
