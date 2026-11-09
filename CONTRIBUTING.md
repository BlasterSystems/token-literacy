# Contributing to token-literacy

Thanks for considering a contribution. This is a small, local-first tool with a deliberately narrow v0.1 scope: Claude Code and Codex only.

## Development setup

- Go 1.24+ (`go version`; CI uses the version in `go.mod`).
- Optional for M5 only: Rust 1.84+ and the Tauri v2 CLI.
- No Node, database server, or Docker required for the collector.

```bash
git clone https://github.com/BlasterSystems/token-literacy
cd token-literacy
make test
make build
make run          # serves http://127.0.0.1:4318/health
```

Validation gates:

```bash
make test
make build
.agentic/bin/validate
.agentic/bin/docs-check
```

## Ground rules

- **Correctness before breadth.** v0.1 measures two harnesses; do not add a third adapter without an accepted ADR.
- **Token semantics are the product.** Every adapter must state whether `input_tokens` includes cache and must satisfy the invariants in `docs/PLAN.md`.
- **No prompt content.** Parsers allow-list usage fields. Never commit prompts, tool output, API keys, subscription tokens, or unredacted home paths.
- **Fixtures over live harnesses.** Add a redacted fixture under `testdata/` and drive it through the production ingest path. CI does not install Claude Code or Codex.
- **One writer per file set.** Keep overlapping edits in one owner when agents are used.
- **Docs and ADRs move with the code.** A durable decision needs an ADR; a user-visible behavior change needs README/plan updates.

## Pull requests

- Branch from `main`; keep changes narrowly scoped.
- Reference the task (`TL-<number>`) and the task file in the PR body.
- Use imperative commit subjects: `feat(scope): ...`, `fix(scope): ...`, `docs: ...`, `chore: ...`.
- Include validation evidence: commands run and observed output.
- Significant changes require an independent adversarial review before merge.
- Direct pushes to `main` are a process violation after the initial scaffold.

## Adding a harness (post-v0.1)

1. Write an ADR comparing the tap point and semantics.
2. Extend the normalized row only if the new source cannot fit it.
3. Add an adapter under `internal/ingest/<harness>` with a golden fixture.
4. Assert the harness invariants and dedupe key.
5. Add the adapter to `tokenledger doctor` and the dashboard filter.

## Reporting issues

- Bugs and feature requests: use the issue templates.
- Security vulnerabilities: do not open an issue; see [SECURITY.md](SECURITY.md).

## License

Contributions are accepted under Apache-2.0. No CLA is required.
