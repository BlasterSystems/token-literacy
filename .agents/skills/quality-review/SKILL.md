---
name: quality-review
description: Review a diff through three lenses - reuse, quality, and efficiency - and report verified, prioritized findings with file references.
---

# Quality Review

Review the current diff (or the pull request the user names) before it is merged. Prefer a fresh, read-only pass that tries to disprove the change's correctness claims. Report findings; do not edit files unless asked.

## Lens 1: Reuse

- An existing helper, module, or pattern already does what new code re-implements.
- New code duplicates logic that should be shared, or diverges from an established convention.
- New dependencies where the standard library or an existing dependency suffices.

## Lens 2: Quality

- Correctness: edge cases, error paths, null or empty inputs, concurrency, idempotency.
- Clarity: naming, function size, dead code, misleading comments.
- Tests: behaviour changes covered, assertions meaningful, failure cases exercised.
- Compatibility: public interfaces, data formats, migrations, configuration.

## Lens 3: Efficiency

- Repeated work in loops (queries, network calls, file reads) that can be batched or hoisted.
- Unbounded reads, missing pagination, or memory growth on large inputs.
- Hot paths doing avoidable allocation, serialization, or I/O.

## Report

List findings most-severe first, each with `path:line`, the failure scenario, and a concrete fix. Separate confirmed defects from suggestions. State explicitly when a lens found nothing.
