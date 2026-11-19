#!/usr/bin/env -S python3 -I
from __future__ import annotations

import hashlib
import json
import os
import re
import sys
import tomllib
import unicodedata
from pathlib import Path, PurePosixPath

# REFRACT_ROOT is set by the pinned agent hook launcher, which runs a verified
# private copy of this script against the project.
ROOT = Path(os.environ.get("REFRACT_ROOT") or Path(__file__).resolve().parents[2])
MANIFEST = ROOT / ".refract/manifest.json"
MANIFEST_SCHEMA = "refract-manifest/v1"
VALID_STATUSES = {"Proposed", "Accepted", "Rejected", "Deprecated", "Superseded"}
TOKEN = re.compile(rb"\{\{[A-Z0-9_]+\}\}")
PORTABLE_FORBIDDEN = ("mcp__", "Task tool", ".claude/", ".codex/", ".opencode/", ".pi/", ".omp/")
ADR_FILENAME = re.compile(r"^(\d{4})-[a-z0-9][a-z0-9-]*\.md$")
ADR_REFERENCE = re.compile(r"ADR-(\d{4})")
REQUIRED_SECTIONS = (
    "## Context", "## Decision drivers", "## Considered options", "## Decision",
    "## Consequences", "## Implementation and validation", "## Revisit triggers", "## References",
)
# User/global agent configuration directories. Generated agent content must
# never direct an agent to install skills, agents, hooks, or settings there.
GLOBAL_SCOPE_PATTERN = re.compile(
    r"(?:~|\$HOME|\$\{HOME\}|%USERPROFILE%)[/\\]+"
    r"\.(?:claude|codex|omp|agents|pi|cursor|gemini|config[/\\]+opencode)(?![A-Za-z0-9_-])"
)
AGENT_SURFACE_PREFIXES = (
    ".agents/skills/", ".claude/skills/", ".claude/agents/", ".omp/agents/",
    ".opencode/agents/", ".opencode/commands/", ".codex/agents/",
)
# AGENTS.md is a user-editable seed; it may legitimately document ~/ paths.
AGENT_SURFACE_FILES = ("CLAUDE.md", ".omp/RULES.md")


def _front_matter(text: str) -> dict[str, str] | None:
    text = text.replace("\r\n", "\n")
    if not text.startswith("---\n"):
        return None
    end = text.find("\n---", 4)
    if end == -1:
        return None
    fields: dict[str, str] = {}
    for line in text[4:end].splitlines():
        key, sep, value = line.partition(":")
        if sep and key and not key.startswith((" ", "\t")):
            fields[key.strip()] = value.strip().strip("\"'")
    return fields


def agent_surface_errors(relative: str, text: str) -> list[str]:
    """Platform-specific contracts for instruction, skill, agent, and command files."""
    if relative not in AGENT_SURFACE_FILES and not relative.startswith(AGENT_SURFACE_PREFIXES):
        return []
    text = text.replace("\r\n", "\n")
    problems = [
        f"{relative} references user/global agent scope ({match.group(0)}); generated agent content must stay project-scoped"
        for match in GLOBAL_SCOPE_PATTERN.finditer(text)
    ]
    parts = relative.split("/")
    if relative.endswith("/SKILL.md") and len(parts) == 4:
        fields = _front_matter(text) or {}
        if fields.get("name") != parts[2] or not fields.get("description"):
            problems.append(f"{relative}: skill front matter name must match its directory and include a description")
    elif relative.endswith(".md") and parts[0] in {".claude", ".omp"} and parts[1] == "agents":
        fields = _front_matter(text) or {}
        if fields.get("name") != parts[-1][:-3] or not fields.get("description"):
            problems.append(f"{relative}: agent front matter needs name matching its file and a description")
    elif relative.endswith(".md") and parts[0] == ".opencode":
        if not (_front_matter(text) or {}).get("description"):
            problems.append(f"{relative}: OpenCode front matter needs a description")
    elif relative.endswith(".toml") and parts[0] == ".codex":
        try:
            data = tomllib.loads(text)
        except ValueError:
            data = {}
        for key in ("name", "description", "developer_instructions"):
            if not isinstance(data.get(key), str) or not data[key].strip():
                problems.append(f"{relative}: Codex agent requires a non-empty {key}")
    return problems


def mirror_errors(texts: dict[str, str]) -> list[str]:
    """Skill mirrors must match the portable copy (omp prefers .claude/skills)."""
    problems = []
    for relative, text in sorted(texts.items()):
        if relative.startswith(".claude/skills/"):
            portable = ".agents/skills/" + relative[len(".claude/skills/"):]
            if portable in texts and texts[portable] != text:
                problems.append(f"{relative} differs from {portable}; edit both copies or re-run refract")
    return problems


ERRORS: list[str] = []


def error(message: str) -> None:
    ERRORS.append(message)


def safe_path(relative: str) -> Path | None:
    if (
        not isinstance(relative, str)
        or not relative
        or "\\" in relative
        or any(unicodedata.category(char)[0] == "C" or unicodedata.category(char) in {"Zl", "Zp"} for char in relative)
    ):
        error(f"unsafe manifest path: {relative!r}")
        return None
    pure = PurePosixPath(relative)
    if pure.is_absolute() or any(part in {"", ".", ".."} for part in pure.parts):
        error(f"unsafe manifest path: {relative!r}")
        return None
    path = ROOT.joinpath(*pure.parts)
    current = ROOT
    for part in pure.parts:
        current = current / part
        if current.is_symlink():
            error(f"manifest path contains symlink: {relative}")
            return None
    return path


def manifest_paths() -> list[Path]:
    if not MANIFEST.is_file() or MANIFEST.is_symlink():
        error("owned .refract/manifest.json is required")
        return []
    try:
        data = json.loads(MANIFEST.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        error(f"invalid ownership manifest: {exc}")
        return []
    if data.get("schema") != MANIFEST_SCHEMA or not isinstance(data.get("files"), dict):
        error("invalid ownership manifest schema")
        return []
    paths = [MANIFEST]
    for relative, metadata in data["files"].items():
        path = safe_path(relative)
        if path is None:
            continue
        if (
            not isinstance(metadata, dict)
            or not isinstance(metadata.get("sha256"), str)
            or len(metadata.get("sha256", "")) != 64
            or not isinstance(metadata.get("executable"), bool)
            or not isinstance(metadata.get("mutable", False), bool)
        ):
            error(f"invalid manifest metadata: {relative}")
            continue
        if not path.is_file() or path.is_symlink():
            error(f"manifest-owned path is missing or not a file: {relative}")
            continue
        if not metadata.get("mutable", False):
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            if digest != metadata["sha256"]:
                error(f"manifest-owned file content changed: {relative}")
            executable = bool(path.stat().st_mode & 0o100)
            if executable != metadata["executable"]:
                error(f"manifest-owned file mode changed: {relative}")
        paths.append(path)
    return paths


def validate_generated(paths: list[Path]) -> None:
    surfaces: dict[str, str] = {}
    for path in paths:
        relative = path.relative_to(ROOT).as_posix()
        if relative in AGENT_SURFACE_FILES or relative.startswith(AGENT_SURFACE_PREFIXES):
            try:
                surfaces[relative] = path.read_text(encoding="utf-8")
            except (OSError, UnicodeDecodeError):
                continue
    for relative, text in sorted(surfaces.items()):
        for problem in agent_surface_errors(relative, text):
            error(problem)
    for problem in mirror_errors(surfaces):
        error(problem)
    for path in paths:
        relative = path.relative_to(ROOT).as_posix()
        try:
            content = path.read_bytes()
        except OSError as exc:
            error(f"{relative}: cannot read: {exc}")
            continue
        if TOKEN.search(content):
            error(f"{relative}: unresolved template token")
        if relative == "AGENTS.md" or relative.startswith(".agents/skills/"):
            try:
                text = content.decode("utf-8")
            except UnicodeDecodeError:
                error(f"{relative}: portable content must be UTF-8 text")
            else:
                for token in PORTABLE_FORBIDDEN:
                    if token in text:
                        error(f"{relative} contains platform-specific token: {token}")
        if path.suffix == ".json":
            try:
                json.loads(content)
            except json.JSONDecodeError as exc:
                error(f"{relative}: invalid JSON: {exc}")
        elif path.suffix == ".toml":
            try:
                tomllib.loads(content.decode("utf-8"))
            except (ValueError, UnicodeDecodeError) as exc:
                error(f"{relative}: invalid TOML: {exc}")
        expected_exec = relative.startswith(".refract/bin/") or relative in {".githooks/pre-commit", "setup-hooks.sh"}
        if expected_exec and not os.access(path, os.X_OK):
            error(f"{relative}: expected executable mode")


def field(text: str, name: str) -> str | None:
    match = re.search(rf"^- \*\*{re.escape(name)}:\*\*\s*(.+?)\s*$", text, re.MULTILINE)
    return match.group(1).strip() if match else None


def references(value: str | None) -> set[str]:
    if value in {None, "None", "—", "-"}:
        return set()
    return set(ADR_REFERENCE.findall(value or ""))


def parse_index(text: str) -> dict[str, tuple[str, str | None]]:
    rows: dict[str, tuple[str, str | None]] = {}
    for line in text.splitlines():
        if not line.lstrip().startswith("|") or "ADR-" not in line:
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) < 2:
            continue
        match = ADR_REFERENCE.search(cells[0])
        if not match:
            continue
        adr_id = match.group(1)
        link_match = re.search(r"\(([^)]+\.md)\)", cells[0])
        if adr_id in rows:
            error(f"ADR index contains duplicate ADR-{adr_id}")
        rows[adr_id] = (cells[1], link_match.group(1) if link_match else None)
    return rows


def validate_adrs() -> None:
    adr_dir = ROOT / "docs/adr"
    index_path = adr_dir / "README.md"
    if not index_path.is_file():
        error("docs/adr/README.md is required")
        return
    index_rows = parse_index(index_path.read_text(encoding="utf-8"))
    records: dict[str, tuple[Path, str, set[str], set[str]]] = {}
    for path in sorted(adr_dir.glob("*.md")):
        if path.name in {"README.md", "template.md"}:
            continue
        match = ADR_FILENAME.match(path.name)
        if not match:
            error(f"{path.relative_to(ROOT)}: expected NNNN-short-title.md")
            continue
        adr_id = match.group(1)
        if adr_id in records:
            error(f"duplicate ADR id: {adr_id}")
            continue
        text = path.read_text(encoding="utf-8")
        status = field(text, "Status") or ""
        supersedes = references(field(text, "Supersedes"))
        superseded_by = references(field(text, "Superseded by"))
        records[adr_id] = (path, status, supersedes, superseded_by)
        if not text.startswith(f"# ADR-{adr_id}:"):
            error(f"{path.relative_to(ROOT)}: heading ID does not match filename")
        if status not in VALID_STATUSES:
            error(f"{path.relative_to(ROOT)}: invalid or missing Status")
        for section in REQUIRED_SECTIONS:
            if section not in text:
                error(f"{path.relative_to(ROOT)}: missing {section}")
    for adr_id, (path, status, supersedes, superseded_by) in records.items():
        row = index_rows.get(adr_id)
        if not row:
            error(f"{path.relative_to(ROOT)}: missing from ADR index")
        else:
            index_status, link = row
            if index_status != status:
                error(f"ADR-{adr_id}: index status does not match file status")
            if link and link != path.name:
                error(f"ADR-{adr_id}: index link does not match filename")
        for ref in supersedes | superseded_by:
            if ref not in records:
                error(f"ADR-{adr_id}: references missing ADR-{ref}")
        if status == "Superseded" and not superseded_by:
            error(f"{path.relative_to(ROOT)}: missing Superseded by link")
        if superseded_by and status != "Superseded":
            error(f"ADR-{adr_id}: Superseded by requires status Superseded")
        for replacement in superseded_by:
            if replacement in records:
                if records[replacement][1] != "Accepted":
                    error(f"ADR-{adr_id}: replacement ADR-{replacement} must be Accepted")
                if adr_id not in records[replacement][2]:
                    error(f"ADR-{adr_id}: replacement ADR-{replacement} does not reciprocally supersede it")
        if status == "Accepted":
            for previous in supersedes:
                if previous in records:
                    if records[previous][1] != "Superseded":
                        error(f"ADR-{adr_id}: superseded ADR-{previous} must have status Superseded")
                    if adr_id not in records[previous][3]:
                        error(f"ADR-{adr_id}: superseded ADR-{previous} does not link back")
    for adr_id, (_status, link) in index_rows.items():
        if adr_id not in records:
            error(f"ADR index references missing ADR-{adr_id}")
        elif link and not (adr_dir / link).is_file():
            error(f"ADR-{adr_id}: index link target does not exist")


agents = ROOT / "AGENTS.md"
if not agents.is_file() or agents.is_symlink():
    error("AGENTS.md must be a real file")
validate_generated(manifest_paths())
validate_adrs()

if ERRORS:
    print("Configuration validation failed:")
    for item in ERRORS:
        # One finding per line: escape control characters from repository paths.
        print("- " + "".join(
            f"\\u{ord(c):04x}" if unicodedata.category(c)[0] == "C" or unicodedata.category(c) in {"Zl", "Zp"} else c
            for c in item
        ))
    sys.exit(1)
print("Configuration validation passed")
