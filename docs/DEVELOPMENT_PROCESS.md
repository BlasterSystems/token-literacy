# Development Process

## Purpose

Use this workflow for changes to **token-literacy**. Scale the ceremony to the risk and size of the task, but do not skip validation or explicit architectural decisions.

## 1. Understand

- Read the request and acceptance criteria.
- Inspect relevant code and current documentation.
- Identify unknowns, risks, and affected components.
- Confirm assumptions that could materially change the solution.

## 2. Plan

- Create or update a task file from `tasks/task-template.md`.
- Break work into independently verifiable steps.
- For multi-step or ambiguous work, use the planning-and-task-breakdown skill; the task list target and tracker mapping are defined in `docs/TASK_BREAKDOWN.md`.
- Record expected tests and documentation changes.
- Decide whether the task has architectural impact.
- For agent-guided bootstrap maintenance, follow `docs/AGENT_GUIDED.md`; the agent stops after preview and a human applies separately.

## 3. Architecture Decision Check

An ADR is normally required for cross-cutting, durable, security-sensitive, data-model, deployment, foundational dependency, or external integration decisions.

If required:

1. Read `docs/adr/README.md`.
2. Load only relevant ADRs.
3. Copy `docs/adr/template.md` to the next available four-digit ID.
4. Complete options, consequences, validation, and revisit triggers.
5. Add the proposed ADR to the index.
6. Obtain the required decision approval before treating it as Accepted.

Routine refactors and local implementation details do not require ADRs.

## 4. Implement

- Keep the change focused and consistent with established patterns.
- Avoid unrelated cleanup.
- Keep one writer for overlapping files when agents are used.
- Never hide failures by weakening tests or security controls.

## 5. Validate

Run applicable commands:

```bash
.agentic/bin/validate
make test
make build
.agentic/bin/docs-check
```

Skip commands marked “Not configured”; record the missing configuration instead of executing that text.

## 6. Review

Review the diff for correctness, security, compatibility, test coverage, documentation, and unnecessary complexity. Update the task file with evidence and residual risks.

Open a pull request for reviewable changes. Before merge, significant changes require an independent adversarial review: a fresh read-only pass that tries to disprove correctness, security, compatibility, and scope claims.

For security-sensitive changes or open vulnerability questions, use the security-audit skill in guidance mode; see `docs/SECURITY_AUDIT.md`.

## 7. Complete

- Ensure acceptance criteria are met.
- Ensure new or superseding ADRs are indexed and linked.
- Confirm generated configuration still validates.
- Provide a concise summary of changes, validation, and remaining risks.
- Before tagging a release that touches security-relevant surfaces, run the scoped release-gate audit described in `docs/SECURITY_AUDIT.md`.
- For a version bump or release, follow `docs/RELEASING.md` with the `semantic-versioning` skill; obtain explicit confirmation before creating the release commit or tag.

## 8. Ticket lifecycle

Work is tracked in the project's issue tracker: task identifiers use the `TL-<number>` convention. **Status transitions are part of the definition of done: a ticket is never left in a state that contradicts the code.**

1. **Fetch** the ticket (description + acceptance criteria) before implementing anything.
2. **Before coding**: transition the ticket to "In Progress" and assign yourself; record the assignment in the ticket.
3. **Plan** in `tasks/<TICKET>.md` from `tasks/task-template.md`; keep it updated with decisions, evidence, and residual risks as the work progresses.
4. **Deliver — always via pull request**: push a branch and open a PR whose title references the ticket and whose body links the ticket and the task file. Direct pushes to the default branch are a process violation — including fixes, hotfixes, docs, and process changes themselves.
5. **On merge — mandatory status update**: transition the ticket to "Done" and post an evidence comment (commands run + observed results). This step is blocking: merging without the status update is a process violation, not optional cleanup.
6. **Multi-ticket changes**: a PR that covers several tickets (e.g. an epic landing its child stories) reconciles every covered ticket's status in the same pass — including epics and parents whose state the change completes. A ticket whose feature is only partially delivered stays "In Progress" with a comment naming the remainder.
7. **Reconciliation**: when work lands at a coarser grain than the tickets describe (an epic PR implementing many stories), verify each covered ticket against the actual code before transitioning it; tickets whose feature is genuinely absent stay open and become the next work items, not silent "Done"s.
