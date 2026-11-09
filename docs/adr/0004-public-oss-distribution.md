# ADR-0004: Public OSS distribution

- **Status:** Accepted
- **Date:** 2026-11-09
- **Decision owners:** Bruno Pires
- **Scope:** License, repository hosting, contribution model, release artifacts, telemetry
- **Tags:** oss, license, github, releases, security, governance
- **Supersedes:** None
- **Superseded by:** None

## Context

The project is a public proof-of-concept for an article on token literacy and AI coding subscriptions. It should be inspectable, reproducible, and safe for readers to run. The GitHub repository `BlasterSystems/token-literacy` exists and is public; its initial commit contains the Apache-2.0 license text.

## Decision drivers

- A permissive license that is familiar to infrastructure and developer-tool users.
- No CLA or copyright assignment friction for casual contributors.
- Reproducible releases with checksums and transparent scope.
- No telemetry, no accounts, no hidden data collection.
- Security reports must have a private path.

## Considered options

### Option 1: Apache-2.0 (selected)

Permissive, patent grant, compatible with the existing BlasterSystems repositories. The remote initial commit already carries the license.

### Option 2: MIT

Simpler and shorter, but no explicit patent grant; switching would require replacing the existing license and notice.

### Option 3: AGPL-3.0

Strong copyleft; mismatched with a local developer tool and the rest of the organization's permissive portfolio.

### Option 4: Source-available / no license

Not OSS; blocks reuse and contribution.

## Decision

Distribute under **Apache-2.0**. Host on public GitHub at `BlasterSystems/token-literacy`. Use GitHub Issues and pull requests; no CLA. Publish versioned artifacts through GitHub Releases with SHA-256 checksums. No telemetry, no analytics, and no outbound network calls from the collector. Security reports go through GitHub private vulnerability reporting, never public issues.

## Consequences

### Positive

- Readers can audit every parser and reproduce the article's numbers.
- Patent grant and clear attribution fit the organization's other repos.
- No CLA lowers contribution friction.
- GitHub Releases keep install simple and versioned.

### Negative

- Public issue volume and support expectations; no SLA.
- Unsigned macOS/Windows builds trigger Gatekeeper/SmartScreen until certificates are available.
- Apache-2.0 requires preserving notices and stating changes; contributors must be aware.

### Risks and mitigations

- **Accidental data publication:** fixtures are redacted; CI grep for `sk-`, tokens, and home paths; no prompt content in the repo.
- **Supply-chain risk:** minimal dependencies; Dependabot; pinned CI actions; checksums on releases.
- **Security reports in public:** SECURITY.md and issue-template contact link point to private reporting.
- **Trademark confusion:** README states the project is not affiliated with Anthropic or OpenAI.

## Implementation and validation

- `LICENSE` present at the repository root; README links it.
- `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue/PR templates committed.
- CI runs on Ubuntu, macOS, and Windows; releases attach archives and checksums.
- Private vulnerability reporting enabled in repository settings before v0.1.0.

## Revisit triggers

- The organization adopts a standard CLA or relicensing policy.
- A hosted/team edition requires a different distribution or license boundary.
- macOS notarization or Windows code signing certificates become available.

## References

- `README.md`, `CONTRIBUTING.md`, `SECURITY.md`
- `docs/RELEASING.md`
- GitHub repository: `https://github.com/BlasterSystems/token-literacy`
