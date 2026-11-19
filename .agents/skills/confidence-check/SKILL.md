---
name: confidence-check
description: Readiness gate before implementation - confirm requirements, blast radius, test strategy, security and data-isolation impact, and rollback before writing code.
---

# Confidence Check

Run before implementation starts, and whenever scope changes mid-task. Report the result; the calling workflow (for example `implement`) records it in the task file.

## Checklist

1. **Requirements**: acceptance criteria are clear; edge cases and constraints are noted.
2. **Impact**: files to change are listed; blast radius and upstream/downstream dependencies are understood.
3. **Tests**: unit and integration coverage is planned, including edge cases.
4. **Security**: input validation, authentication and authorization, injection vectors, secrets, and sensitive-data handling are considered.
5. **Data isolation**: every read and write that must be scoped (tenant, account, user, organization) is scoped, and the scope comes from trusted context rather than client input.
6. **Rollback**: the change can be reverted safely; migrations are reversible or have a documented forward fix; a feature flag is considered for risky rollouts.
7. **Deployment**: deploy-affecting changes name an artifact invariant and a functional probe (see `docs/DEPLOYMENT.md`).

## Red flags: pause and ask

- Unclear acceptance criteria.
- A change that spans several independent concerns: propose splitting it into smaller pull requests.
- No obvious test strategy.
- A scoped query or write without its isolation filter: stop and flag it immediately.
- A durable or cross-cutting decision without an ADR.
