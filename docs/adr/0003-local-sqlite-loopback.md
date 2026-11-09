# ADR-0003: Local SQLite ledger and loopback-only HTTP

- **Status:** Accepted
- **Date:** 2026-11-09
- **Decision owners:** Bruno Pires
- **Scope:** Persistence, network posture, privacy, single-user operation
- **Tags:** sqlite, security, privacy, local-first, http
- **Supersedes:** None
- **Superseded by:** None

## Context

The tool measures one developer's harness usage on their own workstation. It needs durable per-turn rows, a small query surface, and a dashboard. A database server, cloud sync, or account system would add operational and privacy cost with no v0.1 benefit. The data is small (thousands of rows per week), and the dashboard is read-mostly.

## Decision drivers

- Zero setup: no Postgres, Docker, or external service.
- Single-file backup and easy reset.
- No network exposure beyond the local machine.
- No prompt content at rest.
- Idempotent ingest across restarts.

## Considered options

### Option 1: SQLite file (selected)

WAL mode at `~/.tokenledger/ledger.db`; pure-Go driver; per-user file; write on ingest, read on dashboard.

### Option 2: Embedded key-value store

Simpler writes, but weaker ad-hoc queries and reporting; CSV export and filters become application code.

### Option 3: Postgres/MySQL

Better for multi-user, but adds a service, credentials, migrations, and failure modes the product does not need.

### Option 4: In-memory only

No history across restarts; unacceptable for the article and for `doctor`.

## Decision

Use **SQLite (WAL)** at `~/.tokenledger/ledger.db` with a pure-Go driver. Bind the HTTP server to `127.0.0.1` only. No auth in v0.1; reject non-loopback `RemoteAddr` and `Host`. Store usage metadata only: no prompts, tool output, or message bodies. Retain raw per-request rows indefinitely at single-developer volume; `tokenledger uninstall --purge` deletes the database.

## Consequences

### Positive

- No external service; one file to back up, inspect, or delete.
- Idempotent upsert and WAL give crash-safe ingest.
- Local-only network posture is easy to reason about and to document.
- CSV/JSON export is a direct query, not an ETL job.

### Negative

- No multi-user or team mode without a schema/server change.
- SQLite write concurrency is limited; acceptable for one collector process.
- The database can grow with years of history; rollups may be needed later.

### Risks and mitigations

- **Database corruption:** WAL, `PRAGMA integrity_check` in `doctor`, documented reset path.
- **Local HTTP abuse:** loopback bind and host checks; no CORS; no auth token to leak.
- **Privacy drift:** parser allow-lists and tests assert that prompt fields never reach `calls.raw`.

## Implementation and validation

- Schema creation and migrations under an advisory lock or `PRAGMA user_version`.
- Restart test: ingest the same fixture twice; row count unchanged.
- `doctor` reports DB path, size, WAL state, and `integrity_check`.
- Network test: server rejects a non-loopback `Host` and never listens on `0.0.0.0`.

## Revisit triggers

- Team or hosted mode becomes a requirement.
- The ledger exceeds a size where per-request retention needs rollups.
- A second collector process needs concurrent writes.

## References

- `docs/PLAN.md` M1, M3, M4
- ADR-0001 (Go collector)
- ADR-0002 (additive token model)
