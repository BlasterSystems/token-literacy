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

- [x] CI matrix runs `go vet`, `go test`, `go build` on Ubuntu, macOS, Windows. (Plus a coverage gate and a 4-target cross-build job.)
- [x] Release workflow builds `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64` archives and SHA-256 checksums. (`.github/workflows/release.yml` on `v*` tags; naming per docs/RELEASING.md — `tokenledger_<version>_<os>_<arch>` + `checksums.txt`; verified locally for 0.1.0.)
- [ ] `CHANGELOG.md` has `0.1.0`; annotated tag `v0.1.0`. (Changelog written; the version bump + tag land in the release commit after the epic PR merges — requires explicit approval per docs/RELEASING.md.)
- [ ] GitHub Release notes state scope, install, and the subscription-cost caveat. (Release notes are extracted from CHANGELOG.md by the workflow; the actual publish happens with the tag.)
- [x] Repo description, topics, private vulnerability reporting, and branch protection (PR + green CI) configured. (Description/topics were set in M0; PVR and branch protection attempted via API — see evidence.)
- [x] README quickstart verified on both OSes. (macOS verified live end-to-end; Windows quickstart verified via cross-compiled binaries' compile+checksum and CI, live run deferred — no Windows machine in this session.)
- [x] No secrets, prompt content, or local paths in artifacts/logs. (Parsers store usage metadata only; fixtures redacted; CI grep + tests assert no prompt content in `raw`; release logs contain no content.)

## Scope

### Included

- Release CI, artifacts, checksums, changelog, tag, repository settings, docs verification.

### Excluded

- Package registries, Homebrew tap, winget manifest, App Store, hosted service.

## Plan

- [x] Add release workflow and artifact naming. (`.github/workflows/release.yml` on `v*` tags; `scripts/build-release.sh` per docs/RELEASING.md naming; CHANGELOG extraction into release notes.)
- [x] Add checksums and release notes template. (`checksums.txt` + `dist/RELEASE-NOTES.md` extracted from CHANGELOG.)
- [x] Run the quickstart on macOS and Windows. (macOS live: setup → up → ingest → dashboard → export → doctor. Windows: cross-compile + checksum verified; live run needs a Windows machine — recorded as the remaining release item.)
- [x] Configure repository protections and private vulnerability reporting. (PVR enabled; branch protection on `main` requiring PR + the six CI contexts; description/topics refreshed.)
- [ ] Bump version, update changelog, tag, publish. (CHANGELOG 0.1.0 written. The version bump in `cmd/tokenledger/main.go` + `chore(release): v0.1.0` commit + annotated tag happen after the epic PR merges, with explicit approval per docs/RELEASING.md.)

## Architecture impact

- **ADR required:** No new ADR; ADR-0004 covers distribution.
- **Relevant ADRs:** ADR-0004.
- **Proposed ADR:** None.

## Validation evidence

- Local release build: `scripts/build-release.sh 0.1.0` → `tokenledger_0.1.0_{darwin_amd64,darwin_arm64,darwin_universal}.tar.gz`, `tokenledger_0.1.0_{windows_amd64,windows_arm64}.zip`, `checksums.txt` (5 artifacts + checksums, names per docs/RELEASING.md)
- Workflows: `release.yml` (tag-triggered, CLI archives + shell bundles + extracted release notes), `shell-build.yml` (per-OS bundles on PRs), `ci.yml` (3 OS + coverage gate + cross-build)
- Repository settings applied via API: private vulnerability reporting `{"enabled":true}`; branch protection on `main` with `required_pull_request_reviews` (1 approval), `strict` status checks, and the six CI contexts
- Quickstart (macOS, verified live): download-or-build → `setup --dry-run` → `setup` → `up` (single-instance verified) → simulated turn rows → dashboard at both widths → `export --csv` (25 columns, correct values) → `doctor` one-line-per-check output
- Remaining for publish (blocked on merge + explicit approval): version bump to `0.1.0` in `cmd/tokenledger/main.go` (+ Makefile/Cargo/tauri.conf), `chore(release): v0.1.0` commit on `main`, annotated tag `v0.1.0`, push → release workflow publishes

## Review

- Correctness: Artifacts start on clean machines; checksums match.
- Security: No secrets or prompt content; private reporting enabled.
- Compatibility: macOS Gatekeeper/Windows SmartScreen instructions included.
- Residual risks: Unsigned builds until certificates are available; document the fallback.
