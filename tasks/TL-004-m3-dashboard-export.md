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

- [ ] Summary cards: total tokens, hit rate, fresh share, context tax, cost, cost per output token.
- [ ] Per-turn stacked bars (`U/R/W/O`) and a cache-hit-rate line.
- [ ] Session, harness, and date filters; turn table with source and cost label.
- [ ] `GET /api/summary`, `/api/rows`, `/api/sessions`; `GET /export.csv`, `/export.json`.
- [ ] Empty states: no Claude spans, absent Codex directory, unsupported Codex version, WSL warning.
- [ ] No external asset requests; 1280 px and 390 px browser checks; no horizontal overflow.
- [ ] CSV for a filter equals the dashboard's numbers.

## Scope

### Included

- Server-rendered UI, inline SVG, query API, CSV/JSON export, empty states.

### Excluded

- Authentication, multi-user, live SSE, tool-level attribution, Tauri window.

## Plan

- [ ] Implement query API and aggregate SQL.
- [ ] Implement the dashboard layout and two charts.
- [ ] Implement filters and CSV/JSON export.
- [ ] Add empty/error states.
- [ ] Browser-verify light/dark and narrow viewport.

## Architecture impact

- **ADR required:** No new ADR; ADR-0003 covers local-only HTTP.
- **Relevant ADRs:** ADR-0003.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: Metrics derive from the additive row, not `total_tokens`.
- Security: No prompt content; loopback only; no external assets.
- Compatibility: Works in Safari/Chrome/Firefox current versions.
- Residual risks: Inline SVG complexity; test at both widths.
