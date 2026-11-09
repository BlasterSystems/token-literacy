# Task: TL-001 — M0 scaffold, OSS surface, Go walking skeleton

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M0)

## Objective

Establish the governed repository: agentic-bootstrap scaffold, Go + optional Tauri stack decision, public OSS surface, and a buildable walking skeleton so implementation starts from agreed ground truth.

## Product manager summary

- **One-liner:** Create the public, Apache-2.0 repository with a buildable Go collector shell and the full v0.1 plan.
- **User value:** Readers can inspect, build, and later run a local token ledger without a proxy or account.
- **Business impact:** Establishes the proof-of-concept artifact for the token-literacy article.
- **Success metric:** `make test`, `make build`, `.agentic/bin/validate`, and `.agentic/bin/docs-check` pass; the public repo is pushed with OSS files and CI.
- **Dependencies & constraints:** `agentic-bootstrap` in `~/Developer`; Go 1.24+; public repo `BlasterSystems/token-literacy`; Apache-2.0.

## Acceptance criteria

- [x] agentic-bootstrap scaffold applied with all harness platforms; `.agentic/bin/validate` green.
- [x] Go module, minimal `tokenledger serve` with `/health`, `go test`, and `Makefile`.
- [x] `docs/PLAN.md`, `docs/ARCHITECTURE.md`, ADR-0001…0004 accepted and indexed.
- [x] Apache-2.0 LICENSE, README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, NOTICE, issue/PR templates, CI, Dependabot.
- [x] Git remote `BlasterSystems/token-literacy`; branch rebased onto the remote initial LICENSE commit.
- [x] `make test`, `make build`, `.agentic/bin/validate`, `.agentic/bin/docs-check` green.
- [x] Public repository pushed; description and topics set.

## Scope

### Included

- Documentation, ADRs, OSS policy files, CI, Go walking skeleton, remote setup.

### Excluded

- Collector ingest, SQLite ledger, Claude/Codex adapters, dashboard, Tauri shell, release artifacts (M1–M6).

## Plan

- [x] Inspect agentic-bootstrap answers schema and CLI.
- [x] Dry-run and apply the scaffold.
- [x] Rebase the scaffold onto the remote initial commit.
- [x] Write plan, architecture, ADRs, README, OSS files, CI, and Go skeleton.
- [x] Run validation and push.

## Architecture impact

- **ADR required:** Yes.
- **Relevant ADRs:** ADR-0001…0004.
- **Proposed ADR:** Go collector + optional Tauri shell; additive token model; local SQLite + loopback; public OSS distribution.

## Validation evidence

- Configuration validation: `.agentic/bin/validate` → `Configuration validation passed`.
- Tests: `go test ./...` → `internal/server` ok; `cmd/tokenledger` has no test files.
- Build: `make build` → `bin/tokenledger`; `tokenledger version` → `0.1.0-dev`; `/health` returned 200 with `status=ok`.
- Documentation check: `.agentic/bin/docs-check` → exit 0 (`Docs check: diff is empty; perform a manual review if changes are uncommitted or outside Git`).

## Review

- Correctness: The walking skeleton serves `/health`; no collector behavior is claimed.
- Security: Loopback-only server; no prompt content; no outbound calls.
- Compatibility: Go 1.24+; no CGO planned.
- Residual risks: Unsigned binaries will trigger macOS Gatekeeper / Windows SmartScreen until M6.
