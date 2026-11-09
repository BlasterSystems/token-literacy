# ADR-0001: Go collector with an optional Tauri shell

- **Status:** Accepted
- **Date:** 2026-11-09
- **Decision owners:** Bruno Pires
- **Scope:** Runtime language, process model, desktop shell, cross-platform distribution
- **Tags:** go, tauri, desktop, packaging, cross-platform, architecture
- **Supersedes:** None
- **Superseded by:** None

## Context

The product is a local daemon that receives OTLP/HTTP JSON, tails Codex rollout files, stores per-turn token rows in SQLite, and serves a small dashboard. It must install and run on macOS and Windows with minimal ceremony. A tray/menu-bar icon is desirable but the dashboard is already a web page; a full desktop window is not required.

The collector has no native UI requirement, but it does have a long-running process lifecycle, file access in the user's home directory, and a need for reliable cross-compilation from one development machine.

## Decision drivers

- One executable with no runtime dependency is the simplest install.
- `GOOS=windows` / `GOOS=darwin` builds should work from macOS without per-OS CI just to produce a binary.
- SQLite must not require a C toolchain or native module rebuild.
- The dashboard should be testable in a normal browser and embeddable in a future shell.
- A tray icon and signed installers are optional polish, not the core product.

## Considered options

### Option 1: Go collector + browser dashboard

A single Go binary serves OTLP ingest and the dashboard on loopback. `CGO_ENABLED=0` plus a pure-Go SQLite driver removes native dependencies. Cross-compilation is one command per target. No webview, no Chromium, no Node runtime.

### Option 2: Tauri v2 only

Rust backend plus a webview window. Best installer feel and native tray, but requires Rust for the collector, per-OS build runners, and a webview dependency. The browser dashboard is still the main UI, so most of Tauri's value is the tray and installer.

### Option 3: Electron

Fastest UI development and turnkey installers, but a 100 MB+ Chromium runtime for a local token collector, native SQLite rebuilds per platform, and a larger security surface.

### Option 4: Bun/TypeScript single binary

Fastest development for a TypeScript-heavy team and built-in SQLite, but Windows distribution and cross-compiled binary maturity are less proven than Go for an always-on daemon.

### Option 5: .NET/Avalonia

Strong on Windows and familiar to .NET teams, but a larger self-contained runtime and more UI work; cross-platform tray support adds native dependencies.

## Decision

Use **Go 1.24+** for the collector, CLI, HTTP server, SQLite ledger, and server-rendered dashboard. Ship one static binary per OS/arch with `CGO_ENABLED=0` and a pure-Go SQLite driver. Add an **optional Tauri v2 tray shell** in M5 that bundles and supervises the same Go binary; the Go binary remains a complete product without the shell.

## Consequences

### Positive

- One binary, no runtime, per-user install; trivial rollback (delete the binary and login item).
- Cross-compilation from macOS produces macOS and Windows artifacts without a native toolchain.
- The dashboard is testable with `httptest` and a browser; no webview test harness for the core.
- Tauri can be added later without rewriting the collector, schema, or adapters.

### Negative

- Go is a second language for a TypeScript/.NET-heavy workspace.
- A browser tab is less discoverable than a menu-bar icon until M5.
- Windows path and WSL behavior still need explicit testing; Go does not remove that work.

### Risks and mitigations

- **Windows binary maturity:** build and smoke-test all four targets in CI; avoid CGO and native modules.
- **Tauri sidecar signing:** sign the nested Go binary with the same identity; verify with `codesign --verify --deep --strict`.
- **Tray scope creep:** keep the shell thin; no duplicated UI, no second data path.

## Implementation and validation

- `CGO_ENABLED=0 go build` for `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64`.
- `go test ./...` and `go vet ./...` in a three-OS CI matrix.
- Manual smoke: run `tokenledger up` on macOS and Windows; confirm `/health` and dashboard.
- M5 smoke: menu bar/tray icon on both OSes; no Dock icon on macOS; Quit stops the sidecar.

## Revisit triggers

- Go cross-compilation or Windows runtime behavior blocks a release.
- Tauri sidecar/signing complexity outweighs the tray benefit.
- A native window becomes a product requirement rather than polish.

## References

- `docs/PLAN.md` M0, M4, M5
- ADR-0003 (local SQLite and loopback HTTP)
- ADR-0004 (public OSS distribution)
