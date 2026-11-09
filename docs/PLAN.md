# Implementation plan — token-literacy v0.1

Status: **pre-implementation**; M0 scaffold complete. This plan covers the first public release and deliberately excludes every harness except Claude Code and Codex.

## Thesis

Agent bills are dominated by context replay, not output. A single "total tokens" number hides the only three levers that matter: uncached input (`U`), cache reads (`R`), and cache writes (`W`). Claude Code and Codex both expose those fields today; they disagree about whether cache is included in `input_tokens`, and neither exposes a usable local per-turn cache-efficiency view. The product is the normalization and the view, not another counter.

## Non-goals (v0.1)

- No proxy, gateway, or interception of vendor OAuth traffic.
- No LiteLLM, OpenRouter, omp, OpenCode, Cursor, or Copilot adapters.
- No cloud sync, accounts, multi-user mode, or remote ingestion.
- No prompt, tool output, or message-body storage.
- No exact subscription billing; cost is **API-equivalent** and labelled as such.
- No historical Claude Code transcript import (Codex rollouts are backfilled because they are authoritative and local).
- No native app window until M5; the Go binary plus browser dashboard is the complete product.

## Milestones

| M | Task | Theme | Exit |
|---|---|---|---|
| M0 | [TL-001](../tasks/TL-001-m0-scaffold-and-oss.md) | Bootstrap, Go walking skeleton, OSS surface | Validate + docs-check green; public repo pushed |
| M1 | [TL-002](../tasks/TL-002-m1-collector-core-claude.md) | Collector core + Claude Code adapter | One live turn matches the span exactly |
| M2 | [TL-003](../tasks/TL-003-m2-codex-rollout-adapter.md) | Codex rollout adapter | One day reconciles with `ccusage` within 1% |
| M3 | [TL-004](../tasks/TL-004-m3-dashboard-export.md) | Dashboard, filters, CSV/JSON | Per-turn cache view and reproducible export |
| M4 | [TL-005](../tasks/TL-005-m4-setup-doctor-packaging.md) | Setup, doctor, autostart, cross-platform builds | Fresh macOS + Windows install works |
| M5 | [TL-006](../tasks/TL-006-m5-tauri-tray-shell.md) | Optional Tauri v2 tray shell | Menu bar / tray on both OSes; no Dock icon |
| M6 | [TL-007](../tasks/TL-007-m6-public-release.md) | v0.1.0 public release | Signed-or-documented artifacts + GitHub Release |

## M0 — Scaffold and OSS (done)

- `agentic-bootstrap init --platform all` with answered project metadata.
- Go module, minimal `tokenledger serve` with `/health`, `go test`, `Makefile`.
- Apache-2.0 `LICENSE` (from the remote initial commit), public README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, issue/PR templates, CI, Dependabot, `.gitignore`.
- ADR-0001…0004 accepted and indexed.
- **Exit evidence:** `make test`, `make build`, `.agentic/bin/validate`, `.agentic/bin/docs-check`; `BlasterSystems/token-literacy` public with description and topics.

## M1 — Collector core + Claude Code adapter

**Deliverables**

- `internal/ledger`: SQLite schema (`calls`, `sessions`, `ingest_cursor`, `meta`), WAL, idempotent upsert, queries.
- `internal/server`: OTLP/HTTP JSON ingest at `/v1/metrics`, `/v1/logs`, `/v1/traces`; gzip; `/health`; `/api/summary`, `/api/rows`.
- `internal/ingest/otlp`: Claude Code `claude_code.llm_request` span parser; metric fallback `claude_code.token.usage` (`type=input|output|cacheRead|cacheCreation`).
- `internal/pricing`: embedded list prices; API-equivalent cost; unknown model → `cost_source=none`.
- `tokenledger setup`: merge the OTel `env` block into `~/.claude/settings.json` with backup and diff; `--dry-run`; respect `CLAUDE_CONFIG_DIR`.
- `tokenledger doctor`: config present, collector reachable, last span age.
- Invariant checks on ingest; malformed payloads are logged and rejected without killing the server.

**Acceptance**

- Start `tokenledger up`, run one Claude Code turn; a row appears within 5 seconds.
- Row equals the span: `U + R + W == input_tokens + cache_read_tokens + cache_creation_tokens`; `O == output_tokens`.
- Restarting the collector does not duplicate the row (`request_id` primary key).
- `go test ./...` covers OTLP decoding, mapping, invariant rejection, and idempotent upsert with fixtures.
- If beta tracing is disabled, the dashboard shows an "aggregate only" banner instead of pretending to have per-turn rows.

## M2 — Codex rollout adapter

**Deliverables**

- `internal/ingest/codex`: resolve `$CODEX_HOME/sessions` or `~/.codex/sessions`; recursive discovery; 1 s polling; per-file `(inode, offset)` cursor; complete-line reads.
- Parser: `session_meta`, `turn_context`, `token_usage_record`; `event_msg token_count` only as a fallback when no record exists; never both.
- Mapping: `U = input_tokens − cached_input_tokens`, `R = cached_input_tokens`, `W = 0`, `O = output_tokens`, `reasoning = reasoning_output_tokens` (subset, not added), `prompt_total = input_tokens`, `total = total_tokens`.
- Dedupe `response_id` globally; resumed sessions replay into new files; subagents share the parent `session_id` but carry their own `id`/`parent_thread_id`.
- Backfill all existing rollouts on first start; parser misses surface as an unsupported-version warning in `doctor`, never as guessed numbers.
- Optional `--with-codex-otel`: write `[otel.metrics_exporter.otlp-http]` for `codex.turn.cost_microusd`; not required for token counts.

**Acceptance**

- One Codex turn appears within 2 seconds.
- `U + R == input_tokens`; `U + R + O == total_tokens`; `reasoning` is not added to output.
- Replaying a resumed session does not duplicate rows; a subagent row is attributed to `agent_type=subagent`.
- One day of history reconciles with `ccusage` within 1%; every delta is explained (replay, retries, missing pricing).
- Fixtures cover: record + event duplicate, resumed rollout, subagent, older rollout with only `token_count`.

## M3 — Dashboard and export

**Deliverables**

- Server-rendered HTML plus inline SVG; no CDN, no framework, no build step.
- Summary cards: total tokens, cache hit rate, fresh share, context tax, API-equivalent cost, cost per output token.
- Per-turn stacked bars (`U/R/W/O`) and a cache-hit-rate line; session and harness filters; date range.
- Turn table with time, harness, model, `U/R/W/O`, hit %, cost, source.
- `GET /api/summary`, `/api/rows`, `/api/sessions`; `GET /export.csv`; JSON export.
- Empty states: Claude configured but no spans; Codex directory absent; unsupported Codex version; WSL warning.

**Acceptance**

- A 20-turn session renders both charts and the table with no external asset requests.
- CSV reproduces every number shown in the dashboard for the same filter.
- Cache hit rate and context tax are computed from the additive row, not from `total_tokens`.
- Browser check at 1280 px and 390 px; no horizontal page overflow.

## M4 — Setup, doctor, autostart, packaging

**Deliverables**

- `tokenledger setup --dry-run|--force|--with-codex-otel`; backups; uninstall markers; idempotent merge.
- `tokenledger doctor`: config, harness versions, last Claude span, last Codex row, cursor lag, port, DB path, permissions, WSL detection.
- `tokenledger install` / `uninstall`: per-user `launchd` agent on macOS, per-user `HKCU\...\Run` or Startup shortcut on Windows; never a service.
- Single-instance guard; a second `up` opens the existing dashboard.
- `GOOS=darwin|windows GOARCH=amd64|arm64 CGO_ENABLED=0 go build`; macOS universal via `lipo`; release archives with SHA-256.
- `tokenledger export --csv|--json --since`; `tokenledger simulate --fixture` for demos and tests.

**Acceptance**

- Fresh macOS user: `tokenledger setup && tokenledger install && tokenledger up` → one Claude Code turn visible after restarting Claude Code.
- Fresh Windows user: same flow with native PowerShell/Git Bash; no admin rights; uninstall removes the login item and leaves the ledger intact unless `--purge`.
- `doctor` explains each failure in one line; no silent misconfiguration.
- Cross-compiled binaries start on both OSes and pass `go test` in CI.

## M5 — Optional Tauri v2 tray shell

**Deliverables**

- `src-tauri/` Tauri v2 app; tray/menu-bar icon; `ActivationPolicy::Accessory` on macOS; template icon.
- Go binary bundled as a sidecar; spawn/supervise; health poll; restart on crash; kill on quit.
- Window loads `http://127.0.0.1:4318` (the Go-served UI); no duplicated frontend.
- Menu: Open dashboard, Doctor, Copy last session summary, Quit.
- Autostart via `tauri-plugin-autostart`; single-instance plugin.
- Per-OS CI builds (DMG, MSI/NSIS); sign the app **and** the nested Go binary; notarize macOS; document unsigned fallback.

**Acceptance**

- macOS: menu bar icon, no Dock icon, dashboard opens, Quit stops the sidecar.
- Windows: tray icon, dashboard opens, autostart survives login, uninstall clean.
- Killing the Go process causes the shell to restart it and show an error state.
- The Go binary still runs standalone; Tauri is not required for the CLI.

## M6 — Public v0.1.0 release

**Deliverables**

- CI matrix: `ubuntu-latest`, `macos-14`, `windows-latest`; `go vet`, `go test`, `go build`; cross-build artifacts.
- GitHub Release with archives, checksums, and install notes; no package registry in v0.1.
- `CHANGELOG.md` `0.1.0`; annotated tag `v0.1.0`.
- Repo settings: description, topics, private vulnerability reporting, branch protection requiring PR + green CI.
- README quickstart verified on both OSes.

**Acceptance**

- A new user can download the artifact, run `tokenledger setup`, see a Claude Code turn and a Codex turn, and export CSV.
- Release notes state the v0.1 scope and the subscription-cost caveat.
- No secrets, prompt content, or local paths in artifacts or logs.

## Data contract

### Normalized row

| Field | Type | Meaning |
|---|---|---|
| `id` | TEXT PK | `harness:request_id` |
| `harness` | TEXT | `claude-code`, `codex` |
| `session_id` | TEXT | Harness session id |
| `agent_id` | TEXT NULL | Codex subagent id |
| `turn_index` | INTEGER | Order within session |
| `request_id` | TEXT | Claude `request_id` / Codex `response_id` |
| `model` | TEXT | Provider model id |
| `billing_mode` | TEXT | `subscription`, `api_key`, `credits` |
| `input_uncached` | INTEGER | Fresh prompt tokens |
| `cache_read` | INTEGER | Prompt-cache reads |
| `cache_write` | INTEGER | Prompt-cache writes |
| `output` | INTEGER | Completion tokens |
| `reasoning` | INTEGER | Subset of output (Codex); 0 for Claude |
| `prompt_total` | INTEGER | `input_uncached + cache_read + cache_write` |
| `total_tokens` | INTEGER | `prompt_total + output` |
| `cost_usd` | REAL NULL | API-equivalent |
| `cost_source` | TEXT | `list_price`, `provider_reported`, `none` |
| `started_at` | INTEGER | Epoch ms |
| `ended_at` | INTEGER NULL | Epoch ms |
| `duration_ms` | INTEGER NULL | Wall clock |
| `ttft_ms` | INTEGER NULL | Time to first token |
| `stop_reason` | TEXT NULL | Provider stop reason |
| `attempt` | INTEGER NULL | Retry attempt |
| `tool_name` | TEXT NULL | Reserved; v0.1 leaves null |
| `source` | TEXT | `claude_otel`, `codex_rollout`, `codex_otel` |
| `harness_version` | TEXT NULL | Best-effort |
| `raw` | TEXT NULL | Trimmed source metadata, no prompts |

### Invariants

- Claude Code: `input_uncached = input_tokens`, `cache_read = cache_read_tokens`, `cache_write = cache_creation_tokens`, `output = output_tokens`; `prompt_total` must equal the sum reported by the span.
- Codex: `input_uncached + cache_read == input_tokens`; `prompt_total == input_tokens`; `prompt_total + output == total_tokens`; `reasoning` is a subset of `output` and is never added again.
- No row is inserted if it would be negative, zero-total, or fail its harness invariant; the payload is logged with a redacted shape.
- Dedupe: Claude `request_id`; Codex `response_id`. `event_msg token_count` is ignored whenever a `token_usage_record` exists for the file.

### Claude Code mapping

OTel span `claude_code.llm_request` attributes: `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `model`, `request_id`, `session.id`, `query_source`, `duration_ms`, `ttft_ms`, `stop_reason`, `attempt`, `success`.

Setup env block (user-level settings only):

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

### Codex mapping

Rollout record `token_usage_record`:

```json
{"input_tokens":28739,"cached_input_tokens":28032,"output_tokens":11,"reasoning_output_tokens":0,"total_tokens":28750}
```

→ `U=707`, `R=28032`, `W=0`, `O=11`, `prompt_total=28739`, `total=28750`. Adding `input_tokens` and `cached_input_tokens` is the canonical double-count; the parser must never do it.

## CLI and HTTP surface

```
tokenledger setup [--dry-run] [--force] [--with-codex-otel]
tokenledger up [--port 4318] [--no-open]
tokenledger doctor
tokenledger export --csv|--json [--since 7d] [--out FILE]
tokenledger simulate --fixture DIR
tokenledger install | uninstall [--purge]
tokenledger version
```

| Route | Purpose |
|---|---|
| `POST /v1/metrics`, `/v1/logs`, `/v1/traces` | OTLP/HTTP JSON ingest; gzip accepted; 200 `{}` |
| `GET /health` | Version, uptime, DB path, last Claude span, last Codex row, cursor lag |
| `GET /api/summary?since=&harness=` | Aggregates and literacy metrics |
| `GET /api/rows?since=&harness=&session=&limit=&offset=` | Turn rows |
| `GET /api/sessions?since=` | Session list |
| `GET /export.csv`, `/export.json` | Reproducible article dataset |
| `GET /` | Dashboard |

Bind `127.0.0.1` only. No auth in v0.1; reject non-loopback `RemoteAddr` and non-loopback `Host`.

## Testing

- **Unit/golden:** OTLP JSON decoder, Claude span mapping, Codex parser, replay/dedupe, invariant rejection, config merge, pricing.
- **HTTP:** `httptest` for ingest, health, summary, rows, CSV; gzip request.
- **Fixtures:** redacted real payloads under `testdata/otlp/` and `testdata/codex/`; `tokenledger simulate --fixture` loads them through the production ingest path.
- **Cross-platform:** GitHub Actions matrix on Ubuntu, macOS, Windows; `go vet`, `go test`, `go build`.
- **Manual smoke:** one real Claude Code turn and one real Codex turn on macOS and Windows; `doctor` output captured in the task evidence.
- **No live harness in CI:** fixtures are the contract; live tests are release smoke tests.

## Risks

| Risk | Mitigation |
|---|---|
| Claude Code beta tracing changes or is removed | Keep metric fallback; `doctor` detects span absence; pin verified version in docs |
| Codex rollout schema drift | Tolerant parser; unsupported-version error; fixtures per supported version |
| Double counting Codex usage | `token_usage_record` wins; `token_count` only when no record exists |
| Codex replay across resumed files | Global `response_id` dedupe; fixture for resumed session |
| Subscription cost misread as billed | `billing_mode` + `cost_source`; UI labels "API-equivalent" |
| Pricing drift | Embedded table + `~/.tokenledger/pricing.json` override; unknown → `none` |
| Windows/WSL loopback mismatch | Native Windows first; `doctor` detects WSL and prints host-IP/firewall instructions |
| Tauri sidecar signing | Sign the nested Go binary; `codesign --verify --deep --strict` in release checklist |
| macOS Gatekeeper / Windows SmartScreen | Document unsigned fallback; add signing in M6 when certificates are available |

## Open questions

- Exact Codex `token_type` values for `codex.turn.token_usage` (verify in M2 with a live run before relying on the metric).
- Whether `reasoning_output_tokens` is always a subset of `output_tokens` in current Codex builds (source suggests yes; assert on fixtures).
- Whether Claude Code emits a stable `interaction.sequence` across resumed sessions (fallback: order by span start within `session.id`).
- Whether a tray icon is a v0.1 requirement or M5 polish; the Go binary ships without it.
