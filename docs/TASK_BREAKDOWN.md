# Task Breakdown and Tickets

Optional integrations with two MIT-licensed planning skills, used at step 2 (Plan):

- [planning-and-task-breakdown](https://github.com/addyosmani/agent-skills) — turns a spec or clear requirements into a dependency graph, vertical slices, and ordered tasks with acceptance criteria, verification steps, and checkpoints.
- [create-tickets](https://github.com/emiliosheinz/agent-skills) — turns a specification or product document into a prioritized, dependency-aware list of high-level tickets for an external tracker.

## Install

The skills are never generated or auto-installed. Install on demand:

```bash
npx skills add addyosmani/agent-skills --skill planning-and-task-breakdown
npx skills add https://github.com/emiliosheinz/agent-skills --skill create-tickets
```

- The installed skills are unmanaged project content: never add them to `.agentic/` ownership or commit them.
- A per-skill install of `planning-and-task-breakdown` omits the upstream repository's shared `references/` directory; the skill works standalone, but its definition-of-done reference is unavailable (upstream issue #361).
- Both skills are agent-neutral markdown; no platform-specific configuration is required.

## When to use

| Need | Skill | Granularity |
|---|---|---|
| Multi-step or ambiguous implementation work (step 2) | planning-and-task-breakdown | Task-by-task, file-aware |
| Specification or product document becomes a backlog | create-tickets | High-level, product-oriented |

Use `planning-and-task-breakdown` for engineering plans that stay in the repository. Use `create-tickets` only when the output leaves the repository as tracker items; confirm with the user before creating, updating, or closing external tickets — tracker writes are side effects outside this repository.

## Task list target

`planning-and-task-breakdown` records tasks in a task list target:

| Project convention | Task list target |
|---|---|
| Ticket prefix configured (section 8) | One tracker item per task, using the `PREFIX-<number>` convention; record the tracker location in `tasks/plan.md` |
| No ticket prefix | The skill's defaults: `tasks/plan.md` for the plan document and `tasks/todo.md` for the ordered checklist |

- Implementation detail for each task still lives in a per-task file created from `tasks/task-template.md`; the plan document and task list are the ordered index, not replacements.
- Never overwrite a plan or task list that still contains unchecked tasks for different work; stop and ask the user.
- Checkpoints from the skill map to review moments: run validation and request human review before proceeding past a checkpoint.
