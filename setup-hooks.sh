#!/usr/bin/env bash
# Install refract's pre-commit hook as a pinned launcher.
#
# The launcher lives in git's own hooks directory, which sandboxed coding
# agents cannot write (Codex keeps .git read-only). It records the SHA-256 of
# every workspace script the hook runs and refuses to run the hook if any of
# them changed. Review the changes and re-run this script to re-pin.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

FORCE=false
if [[ "${1:-}" == "--force" ]]; then
  FORCE=true
fi

CURRENT="$(git config --get core.hooksPath || true)"
if [[ -n "$CURRENT" && "$CURRENT" != ".githooks" && "$FORCE" != true ]]; then
  echo "Existing core.hooksPath is '$CURRENT'. Refusing to replace it."
  echo "Integrate .githooks/pre-commit manually or rerun with --force."
  exit 2
fi
if [[ "$CURRENT" == ".githooks" || "$FORCE" == true ]]; then
  # Older refract versions pointed hooksPath at the agent-writable .githooks/.
  git config --unset core.hooksPath || true
fi

HOOKS_DIR="$(git rev-parse --git-path hooks)"
HOOK="$HOOKS_DIR/pre-commit"
MARKER="# refract pinned pre-commit launcher"
if [[ -e "$HOOK" ]] && ! grep -qF "$MARKER" "$HOOK" && [[ "$FORCE" != true ]]; then
  echo "An existing $HOOK is not managed by refract. Refusing to replace it."
  echo "Integrate .githooks/pre-commit manually or rerun with --force."
  exit 2
fi

chmod +x .githooks/pre-commit .refract/bin/validate .refract/bin/docs-check

PINS="$(python3 -I - <<'PY'
import hashlib, json
paths = [".githooks/pre-commit", ".refract/bin/validate", ".refract/bin/docs-check", ".refract/config.json"]
print(json.dumps({p: hashlib.sha256(open(p, "rb").read()).hexdigest() for p in paths}, sort_keys=True))
PY
)"

mkdir -p "$HOOKS_DIR"
cat > "$HOOK.tmp" <<HOOK_EOF
#!/bin/sh
$MARKER (installed by setup-hooks.sh; re-run it to re-pin)
exec python3 -I -c '
import hashlib, json, os, subprocess, sys, tempfile
pins = json.loads(sys.argv[1])
root = os.getcwd()
verified = {}
for path, digest in sorted(pins.items()):
    try:
        data = None if os.path.islink(path) else open(path, "rb").read()
    except OSError:
        data = None
    if data is None or hashlib.sha256(data).hexdigest() != digest:
        print("refract pre-commit: " + path + " changed since setup-hooks.sh pinned it.")
        print("Review the change, then re-run ./setup-hooks.sh to re-pin. Commit blocked.")
        sys.exit(1)
    verified[path] = data
with tempfile.TemporaryDirectory() as private:
    copies = {}
    for path, data in verified.items():
        copies[path] = os.path.join(private, os.path.basename(path))
        with open(copies[path], "wb") as handle:
            handle.write(data)
        os.chmod(copies[path], 0o700)
    env = dict(os.environ, REFRACT_ROOT=root, REFRACT_BIN=private, REFRACT_CONFIG=copies[".refract/config.json"])
    sys.exit(subprocess.call(["bash", copies[".githooks/pre-commit"]], env=env))
' '$PINS'
HOOK_EOF
chmod +x "$HOOK.tmp"
mv "$HOOK.tmp" "$HOOK"
echo "Pinned pre-commit hook installed at $HOOK"
