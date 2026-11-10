# Task: TL-004 — M3 dashboard and export

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M3)

## Objective

Serve a dependency-free dashboard that makes per-turn cache economics legible, plus CSV/JSON export for reproducible article charts.

## Product manager summary

- **One-liner:** Show per-turn `U/R/W/O`, cache hit rate, context tax, and API-equivalent cost in one local page.
- **User value:** Turns raw token rows into a decision surface: where the context bill comes from and whether caching paid off.
- **Business impact:** The article's primary visual evidence.
- **Success metric:** A 20-turn session renders correctly and its CSV reproduces every displayed number.
- **Dependencies & constraints:** TL-002 and TL-003; server-rendered HTML + inline SVG; no CDN.

## Acceptance criteria

- [x] Summary cards: total tokens, hit rate, fresh share, context tax, cost, cost per output token.
- [x] Per-turn stacked bars (`U/R/W/O`) and a cache-hit-rate line.
- [x] Session, harness, and date filters; turn table with source and cost label.
- [x] `GET /api/summary`, `/api/rows`, `/api/sessions`; `GET /export.csv`, `/export.json`.
- [x] Empty states: no Claude spans, absent Codex directory, unsupported Codex version, WSL warning.
- [x] No external asset requests; 1280 px and 390 px browser checks; no horizontal overflow. (`scrollWidth == innerWidth` at both widths; light and dark captured; the only non-relative hrefs are the local export links.)
- [x] CSV for a filter equals the dashboard's numbers. (`TestExportCSVMatchesRows` compares every row's id/U/total between `/export.csv?harness=codex` and `/api/rows?harness=codex&order=asc`.)

## Scope

### Included

- Server-rendered UI, inline SVG, query API, CSV/JSON export, empty states.

### Excluded

- Authentication, multi-user, live SSE, tool-level attribution, Tauri window.

## Plan

- [x] Implement query API and aggregate SQL. (TL-002/003 surface reused; hit rate, fresh share, context tax derive from the additive row only.)
- [x] Implement the dashboard layout and two charts. (`internal/server/dashboard.go`; inline SVG stacked bars + hit-rate polyline; `prefers-color-scheme` light/dark.)
- [x] Implement filters and CSV/JSON export. (`/export.csv` and `/export.json` share the dashboard's filter; export is ascending and unpaginated for reproducibility.)
- [x] Add empty/error states. (no-rows empty state; Claude aggregate-only banner; Codex dir absent; `codex_last_warning` for unsupported/malformed rollouts; WSL loopback note.)
- [x] Browser-verify light/dark and narrow viewport. (Chromium at 1280x900 and 390x844; both themes; zero horizontal overflow.)

## Extra delivered

- `tokenledger simulate --fixture DIR` (brought forward from M4) so fixtures load through the production ingest path — used for the 20-turn browser check and for tests.

## Architecture impact

- **ADR required:** No new ADR; ADR-0003 covers local-only HTTP.
- **Relevant ADRs:** ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation: `.agentic/bin/validate` → "Configuration validation passed"
- Tests: `go test ./...` → ok (9 packages); `make cover` total **77.8%** (gate 75%)
- Build: `make build` → `bin/tokenledger`
- Documentation check: `.agentic/bin/docs-check` → exit 0
- Browser verification (Chromium, `tokenledger up --port 43181` + 20-turn simulated session):
  - 1280x900: `scrollWidth == innerWidth == 1280` (no horizontal overflow); both charts and 20-row table rendered; light and dark themes captured
  - 390x844: `scrollWidth == innerWidth == 390`; cards stack; charts scale responsively
  - Summary: 406,330 total tokens, 87.4% hit rate, 8.3% fresh share, 42.09 context tax, $0.4081 API-equivalent cost, $0.000043/output token
- CSV↔dashboard equality is a permanent test (`TestExportCSVMatchesRows`); additive invariant preserved in JSON export (`TestExportJSON`).

## Review

- Correctness: Metrics derive from the additive row, not `total_tokens`.
- Security: No prompt content; loopback only; no external assets.
- Compatibility: Works in Safari/Chrome/Firefox current versions.
- Residual risks: Inline SVG complexity; test at both widths.
