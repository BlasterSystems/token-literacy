# Branch Documents

Each working branch gets a living business document at `docs/branches/<branch-name>.md`, maintained by the `commit` skill on every commit. It explains the change to reviewers and future readers in plain language; it is not a git log.

## Sections

1. **Business context**: why the change exists and who it serves.
2. **What changed**: user-facing and operator-facing behaviour.
3. **Business rules and constraints**: rules the change enforces or relies on.
4. **Edge cases handled**.
5. **Integration impact**: other systems, contracts, or teams affected.
6. **Out of scope and follow-ups**.
7. **Verification**: how the change was tested, with commands and results.
8. **Changelog**: one entry per commit, newest first.

Update sections in place as the branch evolves; keep the changelog append-only.
