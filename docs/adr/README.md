# Architecture Decision Records

Read this index before loading individual ADRs. Load only records relevant to the current scope, component, tag, or question.

## Lifecycle

Valid statuses are `Proposed`, `Accepted`, `Rejected`, `Deprecated`, and `Superseded`.

Accepted records are historical. Create a new ADR to replace a decision, mark the old record `Superseded`, and link both records.

## Index

| ADR | Status | Scope | Tags | Summary | Supersedes |
|---|---|---|---|---|---|

## When an ADR is required

Use an ADR for durable, cross-cutting, security-sensitive, data, deployment, foundational dependency, or external integration decisions. Routine refactors and local implementation details normally do not require one.

Copy `template.md` to `NNNN-short-title.md`, choose the next unused four-digit number, and update this index.
