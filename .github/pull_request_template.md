## Summary

What changed, and why.

## Task

- Task: `TL-<number>` (`tasks/TL-<number>-<slug>.md`)
- ADR (if durable decision): `docs/adr/NNNN-...`

## Validation

Commands run and observed results:

```text
make test
make build
.agentic/bin/validate
.agentic/bin/docs-check
```

## Privacy and security

- [ ] No prompts, tool output, API keys, subscription tokens, or unredacted personal paths are added.
- [ ] No new outbound network calls, telemetry, or non-loopback listeners.
- [ ] Token semantics changed? The invariants in `docs/PLAN.md` are updated and tested.

## Risks

Residual risks, follow-ups, or explicit non-goals.

## Checklist

- [ ] The change is narrowly scoped and consistent with existing conventions.
- [ ] Tests cover the changed behavior or the change is documentation-only.
- [ ] User-visible documentation is updated.
- [ ] The task file and ADR index are updated.
