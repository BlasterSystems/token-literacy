# Security Audit

Optional integration with the [security-audit skill](https://github.com/cloudflare/security-audit-skill), a six-phase vulnerability audit workflow: reconnaissance, coverage-led hunting, candidate validation, structured findings, independent verification, and a target-neutral report.

The skill has two modes:

- **Guidance mode** (default): security questions, methodology, triage, and focused reviews for the current task. No workflow launch and no artifacts.
- **Full audit mode**: only on an explicit audit, pen-test, or full security-review request. Runs all six phases and writes report artifacts.

## Install

The skill is never generated or auto-installed. Install on demand:

```bash
npx skills add https://github.com/cloudflare/security-audit-skill --skill security-audit
```

- Requires Node.js (the skill ships zero-dependency validators) and an agent that supports parallel sub-agents.
- If the skills CLI does not support the active platform, copy `skills/security-audit/` from the repository into that platform's project skill directory.
- The installed skill is unmanaged project content: never add it to `.agentic/` ownership or commit it.

## When to run

| Moment | Mode and profile | Scope |
|---|---|---|
| Security question during steps 1–6 | Guidance | Current task |
| Security-sensitive review in step 6 | Guidance, focused agents allowed | Current diff |
| Release gate in step 7 | Full audit, `quick` profile | Diff between the last release ref and HEAD |
| Epic or milestone boundary | Full audit, `standard` or `deep` | Whole target |

Full audits are expensive and never per-change: one run finds roughly half of what repeated runs find, and runs are additive because reruns read prior coverage ledgers and findings to target gaps. Never describe one run as complete coverage.

## Execution boundaries

- Run target-controlled builds, tests, processes, browsers, emulators, and fuzzers only inside an OS-enforced sandbox with no external network, an empty allowlisted environment, resource limits, and writes restricted to assigned scratch paths. Without those controls the skill keeps leads as `needs_validation`; never weaken the environment to force confirmation.
- The default output directory is `~/security-audit-skill/<repo>/run-<N>`, outside the repository. Write inside the repository only into a version-control-ignored directory, and never commit audit artifacts.
- The skill does not modify target source. Confirmed findings enter the normal process at step 2 (Plan). A finding that changes a security boundary or data, deployment, or dependency decision requires an ADR in step 3.
- Severity applies only to `confirmed` records; `needs_validation` records state the exact unresolved fact and carry no severity. Do not report unresolved records as vulnerabilities.

## Budget

For large targets, record `budget` (maximum agent invocations) in `run-metadata.json`. Profiles: `quick` (one hunter wave, one critic, one verifier per candidate), `standard` (workflow as written), `deep` (critic waves to a clean pass, separate candidate and record verification). A scoped run must present itself as partial coverage.
