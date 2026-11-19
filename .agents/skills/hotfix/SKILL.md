---
name: hotfix
description: Emergency production fix - minimal change on a hotfix branch from the default branch, full checks, expedited pull request, then deployment through deploy-dev and deploy-prd. Run only when the user explicitly asks.
---

# Hotfix

Run only when the user explicitly asks for a hotfix. A hotfix shortens waiting, never checks.

1. **Start from the default branch**: check it out, `git pull --ff-only`, and create `hotfix/<ticket-or-date>-<short-description>`, using the ticket key when one exists or the UTC date otherwise.
2. **Record the incident** in `tasks/INCIDENT-<ticket-or-date>.md` from `tasks/task-template.md`: symptom, impact, start time, suspected cause.
3. **Implement the smallest fix** that resolves the defect. No refactoring, no adjacent changes, no features.
4. **Add a regression test** that fails without the fix.
5. **Run the checks**: `.refract/bin/validate`, the configured test and build commands, and the `security-check` skill.
6. **Commit** with the `commit` skill.
7. **Confirm with the user**, then push the branch and open an expedited pull request titled `fix: <description> (HOTFIX)` that references the ticket and links the incident file.
8. **Deploy after merge** only through `deploy-dev` and then `deploy-prd`. Both run `.refract/bin/deploy-ready` and ask for confirmation.
9. **Follow up** in the incident file: root cause, verification evidence, and preventive actions (tests, monitoring, an ADR if a durable decision changed).
