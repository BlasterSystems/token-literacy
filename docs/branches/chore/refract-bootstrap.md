# chore/refract-bootstrap

## Business context

The project was bootstrapped with `agentic-bootstrap 0.2.0`. This branch migrates the agent
workflow to **refract 0.3.1** (the successor tool) while keeping every pre-existing file
owned by the project.

## What changed

- Added the nine refract workflow skills (`research`, `confidence-check`, `implement`,
  `security-check`, `quality-review`, `commit`, `deploy-dev`, `deploy-prd`, `hotfix`) plus
  `refract` and the existing four skills remain untouched, in `.agents/skills/` and
  `.claude/skills/`.
- Added `.refract/` control directory: `config.json`, `manifest.json`, and the deterministic
  `bin/validate`, `bin/docs-check`, `bin/agent-check`, `bin/deploy-ready` launchers.
- Added omp platform files: `.omp/agents/architecture-reviewer.md`,
  `.omp/hooks/post/refract-check.ts`, `.omp/mcp.json`, `.omp/settings.json`.
- Added permission policies (`.claude/settings.json`), GitHub PR template
  (`.github/PULL_REQUEST_TEMPLATE.md`), git hook (`.githooks/pre-commit`),
  `setup-hooks.sh`, `docs/DEPLOYMENT.md` seed, and `docs/branches/`.
- No existing file was modified or deleted; 23 pre-existing files at generated paths were
  kept as project-owned (including `AGENTS.md`, `CLAUDE.md`, `.mcp.json`,
  `.github/workflows/docs-check.yml`).

## Business rules and constraints

- Refract is the deterministic bootstrap/validation authority; `.refract/manifest.json`
  records ownership and file hashes.
- The pre-commit hook (installed via `./setup-hooks.sh`) is a hash-pinned launcher: it runs
  `.refract/bin/validate`, `docs-check`, the configured test command (`make test`), and the
  build command (`make build`) on every commit, and blocks if any pinned script changed.
- Commits must not go directly to the default branch.

## Edge cases handled

- Plan identifier (`e51bc71f…e76c`) verified at apply time; apply aborts on any mismatch.
- Claude Code MCP config (`.mcp.json`) was kept project-owned, so refract did not install
  its own MCP block.

## Integration impact

- `docs/DEPLOYMENT.md` is a seed; deploy skills stay inactive until it is filled in and
  `.refract/bin/deploy-ready` passes.
- Refract's docs-check now guards the ADR index and task conventions.

## Out of scope and follow-ups

- Fill in `docs/DEPLOYMENT.md` and set `Status: CONFIGURED`.
- Consolidate `.github/PULL_REQUEST_TEMPLATE.md` with the existing lowercase
  `pull_request_template.md`.
- `.serena/` local tooling state intentionally left uncommitted.

## Verification

- `refract inspect` (bounded walk), dry-run plan review, then apply with
  `--expected-plan e51bc71f656a12d9949c6a61142f7ebcb64ae4ace9af05c8139e35a10986e76c`:
  deterministic result `applied: true`, zero errors.
- Post-apply validation and standalone `.refract/bin/validate`: passed.
- `git status --porcelain`: zero modified tracked files; additions only.

## Changelog

- 2026-11-19 — Bootstrapped refract 0.3.1 agent workflow (additive retrofit,
  `--keep-existing`); installed pinned pre-commit hook.
