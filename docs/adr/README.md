# Architecture Decision Records

Read this index before loading individual ADRs. Load only records relevant to the current scope, component, tag, or question.

## Lifecycle

Valid statuses are `Proposed`, `Accepted`, `Rejected`, `Deprecated`, and `Superseded`.

Accepted records are historical. Create a new ADR to replace a decision, mark the old record `Superseded`, and link both records.

## Index

| ADR | Status | Scope | Tags | Summary | Supersedes |
|---|---|---|---|---|---|
| [ADR-0001](0001-go-collector-optional-tauri-shell.md) | Accepted | Runtime language, process model, desktop shell, cross-platform distribution | go, tauri, desktop, packaging, cross-platform, architecture | Go collector as the product; optional Tauri v2 tray shell in M5; one static binary per OS/arch | None |
| [ADR-0002](0002-additive-token-model-otlp-and-rollout.md) | Accepted | Token semantics, Claude Code and Codex ingestion, dedupe, cost labelling | tokens, cache, opentelemetry, codex, claude-code, data-model | Claude OTel spans plus Codex rollout JSONL; additive `U/R/W/O` row; subscription cost is API-equivalent | None |
| [ADR-0003](0003-local-sqlite-loopback.md) | Accepted | Persistence, network posture, privacy, single-user operation | sqlite, security, privacy, local-first, http | SQLite WAL at `~/.tokenledger/ledger.db`; loopback-only HTTP; no prompt content | None |
| [ADR-0004](0004-public-oss-distribution.md) | Accepted | License, repository hosting, contribution model, release artifacts, telemetry | oss, license, github, releases, security, governance | Apache-2.0 public OSS, no CLA, GitHub Releases, no telemetry, private vulnerability reporting | None |

## When an ADR is required

Use an ADR for durable, cross-cutting, security-sensitive, data, deployment, foundational dependency, or external integration decisions. Routine refactors and local implementation details normally do not require one.

Copy `template.md` to `NNNN-short-title.md`, choose the next unused four-digit number, and update this index.
