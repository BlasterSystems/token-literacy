---
name: commit
description: Commit the current work on a feature branch with a conventional commit message, keeping the living branch document in docs/branches up to date; pushes only when the user asks.
---

# Commit

## Steps

1. **Check the branch.** If the current branch is the default branch (or any branch the process protects), stop: create a feature branch named per the project's conventions, or ask the user. Never commit or push directly to the default branch.
2. **Review everything that will be committed**: `git status`, `git diff`, and `git diff --staged`. Files staged earlier are part of the commit too.
3. **Keep secrets out.** Unstage and warn about credentials, `.env` files, keys, local or personal settings, and generated secrets, whether staged earlier or not. Apply the `security-check` skill to the staged changes and stop on any CRITICAL finding.
4. **Validate**: `.refract/bin/validate` and the configured test command. Tell the user before committing anything that fails.
5. **Update the branch document** at `docs/branches/<branch-name>.md` (see `docs/branches/README.md`): update the affected sections and add one changelog entry, newest first. It is a living business document, not a git log.
6. **Stage explicitly**, file by file.
7. **Write the message** from the staged diff: `type(scope): summary`, with types such as `feat`, `fix`, `refactor`, `docs`, `test`, `chore`. Reference the ticket key when the project uses one, and add any attribution the project's rules require.
8. **Commit.**
9. **Push only when the user asked for it** (or the calling workflow, such as `implement` opening a pull request, requires it), and only the current feature branch. Never force-push without explicit confirmation.
10. **Report** the commit hash, branch, files changed, whether it was pushed, and the branch-document sections touched.
