# Releasing

Use the `semantic-versioning` skill for version bumps, releases, and Git tags.

## Release contract

- Versions follow Semantic Versioning: `MAJOR.MINOR.PATCH`.
- Release tags are annotated and use `vMAJOR.MINOR.PATCH`.
- The changelog records user-visible changes.
- Tests and configured validators must pass before tagging.
- Publishing commits, tags, or packages requires separate explicit approval.

## Repository configuration

1. **Authoritative version:** `cmd/tokenledger/main.go` (`var version`). `make build VERSION=x.y.z` overrides the build-time value for local artifacts; the source constant is the release version.
2. **Derived references:** `CHANGELOG.md` and the README quickstart. There are no generated version files.
3. **Required commands:** `make test`, `make build`, `.agentic/bin/validate`, `.agentic/bin/docs-check`; CI runs `go vet`, `go test`, and `go build` on Ubuntu, macOS, and Windows.
4. **Release branch:** `main`. Before v0.1.0, protect it with a pull-request requirement and the CI check; the initial scaffold predates that protection.
5. **Registry/deployment:** GitHub Releases only in v0.1. No package registry, container image, or hosted service.

Artifact naming for v0.1: `tokenledger_<version>_<os>_<arch>.tar.gz` on macOS and `.zip` on Windows, plus `checksums.txt`. The Go binary is built with `CGO_ENABLED=0`; the optional Tauri shell (M5) is a separate per-OS artifact.

Do not create a tag until these facts are known. Keep this file focused and link to detailed deployment documentation rather than duplicating it.

## Release checklist

1. Start from a clean Git worktree.
2. Review changes since the latest `v*` tag.
3. Propose and confirm the SemVer bump.
4. Update authoritative version sources and `CHANGELOG.md`.
5. Refresh generated artifacts.
6. Run all documented validation.
7. Review and confirm the release diff.
8. Create `chore(release): vX.Y.Z`.
9. Create annotated tag `vX.Y.Z`.
10. Verify the tagged version; push or publish only when explicitly requested.
