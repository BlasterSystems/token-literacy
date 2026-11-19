#!/usr/bin/env -S python3 -I
"""Deterministic readiness gate for the deploy skills.

Exits non-zero, listing what is missing, unless docs/DEPLOYMENT.md declares
"Status: CONFIGURED" and no "Not configured" marker remains in it. The
deploy-dev, deploy-prd, and hotfix skills run this before any merge, tag,
or push, so readiness does not depend on how a model reads prose.
"""
from __future__ import annotations

import os
import re
import sys
from pathlib import Path

ROOT = Path(os.environ.get("REFRACT_ROOT") or Path(__file__).resolve().parents[2])
DEPLOYMENT = ROOT / "docs/DEPLOYMENT.md"
STATUS = re.compile(r"^Status:\s*(\S+)\s*$", re.MULTILINE)


def problems(text: str) -> list[str]:
    found: list[str] = []
    statuses = STATUS.findall(text)
    if statuses != ["CONFIGURED"]:
        found.append('docs/DEPLOYMENT.md must contain exactly one "Status: CONFIGURED" line')
    section = "document"
    for number, line in enumerate(text.splitlines(), start=1):
        if line.startswith("## "):
            section = line[3:].strip()
        if "not configured" in line.lower():
            found.append(f"docs/DEPLOYMENT.md:{number}: {section} is not configured")
    return found


def main() -> int:
    try:
        text = DEPLOYMENT.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"Deployment is not ready: cannot read docs/DEPLOYMENT.md ({exc})")
        return 1
    found = problems(text)
    if found:
        print("Deployment is not ready:")
        for item in found:
            print(f"- {item}")
        return 1
    print("Deployment configuration is ready")
    return 0


if __name__ == "__main__":
    sys.exit(main())
