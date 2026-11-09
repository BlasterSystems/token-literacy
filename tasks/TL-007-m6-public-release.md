# Task: TL-007 — M6 public v0.1.0 release

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M6)

## Objective

Ship v0.1.0 as a reproducible public release: cross-platform artifacts, checksums, release notes, repository protections, and a quickstart verified on both operating systems.

## Product manager summary

- **One-liner:** Publish the first usable macOS/Windows build of the token ledger with a verified quickstart.
- **User value:** Download, configure, see a Claude Code turn and a Codex turn, export CSV.
- **Business impact:** The article can link to a working artifact rather than a repository snapshot.
- **Success metric:** A new user completes the quickstart on macOS and Windows; artifacts and checksums are attached to the GitHub Release.
- **Dependencies & constraints:** TL-002…TL-006; signing certificates if available; Apache-2.0.

## Acceptance criteria

- [ ] CI matrix runs `go vet`, `go test`, `go build` on Ubuntu, macOS, Windows.
- [ ] Release workflow builds `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64` archives and SHA-256 checksums.
- [ ] `CHANGELOG.md` has `0.1.0`; annotated tag `v0.1.0`.
- [ ] GitHub Release notes state scope, install, and the subscription-cost caveat.
- [ ] Repo description, topics, private vulnerability reporting, and branch protection (PR + green CI) configured.
- [ ] README quickstart verified on both OSes.
- [ ] No secrets, prompt content, or local paths in artifacts/logs.

## Scope

### Included

- Release CI, artifacts, checksums, changelog, tag, repository settings, docs verification.

### Excluded

- Package registries, Homebrew tap, winget manifest, App Store, hosted service.

## Plan

- [ ] Add release workflow and artifact naming.
- [ ] Add checksums and release notes template.
- [ ] Run the quickstart on macOS and Windows.
- [ ] Configure repository protections and private vulnerability reporting.
- [ ] Bump version, update changelog, tag, publish.

## Architecture impact

- **ADR required:** No new ADR; ADR-0004 covers distribution.
- **Relevant ADRs:** ADR-0004.
- **Proposed ADR:** None.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: Artifacts start on clean machines; checksums match.
- Security: No secrets or prompt content; private reporting enabled.
- Compatibility: macOS Gatekeeper/Windows SmartScreen instructions included.
- Residual risks: Unsigned builds until certificates are available; document the fallback.
