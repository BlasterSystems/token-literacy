# Deployment

Status: UNCONFIGURED

<!-- Fill in every section, replace every "Not configured" marker, then change the status line above to "Status: CONFIGURED". `.refract/bin/deploy-ready` fails until both are done, and the deploy-dev and deploy-prd skills (which hotfix deploys through) refuse to merge, tag, or push while it fails. This file is project-owned: refract never overwrites your edits. -->

## Environments

| Environment | Skill | Deploy from | Trigger | Tag pattern | Tag regex | Pipeline |
|---|---|---|---|---|---|---|
| dev | `deploy-dev` | Any pushed branch | Tag push | `dev-{YYMMDD}-{N}` | `^dev-[0-9]{6}-[0-9]+$` | Not configured |
| prd | `deploy-prd` | Default branch, after merge | Tag push | `prd-{YYMMDD}-{N}` | `^prd-[0-9]{6}-[0-9]+$` | Not configured |

- `{YYMMDD}` is the current UTC date; `{N}` is one more than the numerically highest existing tag for that date and environment (start at 1).
- Remote: the git remote tags are fetched from and pushed to. Not configured (usually `origin`).
- Pipeline: the CI/CD workflow that the tag triggers, and the command used to watch it (for example `gh run watch`).

## Merge strategy

How `deploy-prd` merges a pull request into the default branch: `merge`, `squash`, or `rebase`, and whether an approving review is required (default: required). Not configured.

## Pre-deploy gates

Checks that must pass before a tag is pushed, for example: required CI checks green, `.refract/bin/validate`, and the configured test and build commands. Not configured.

## Post-deploy verification

- **Artifact invariants**: what must be true of the deployed artifact (bundle contents, versions, configuration keys). Not configured.
- **Functional probes**: smoke checks against the deployed environment, with the exact command. Not configured.
- **Evidence**: where results are recorded, for example a ticket comment with commands and observed output. Not configured.

A configuration change made outside the pipeline (console, CLI, secret wiring) is a deploy: re-run the functional probes afterwards.

## Rollback

How to return an environment to the previous good state (redeploy the previous tag, revert, feature flag), and who may do it. Not configured.

## Deploy-affecting changes

A change is deploy-affecting when it alters what the pipeline builds, packages, configures, or migrates. Before such a change merges, its task file and pull request must name:

1. the **artifact invariant** that proves the deployed artifact is correct, and
2. the **functional probe** that proves the environment works after the deploy,

and the deployment must record both results as evidence.
