---
name: security-check
description: Pre-commit security review of changed files - secrets, injection, authorization, data isolation, error and log leakage - reported as CRITICAL, WARNING, and INFO findings.
---

# Security Check

Review the staged and changed files before committing. Report only what you verified in the diff or the code it touches.

## CRITICAL: must be fixed before commit

1. **Secrets**: hard-coded passwords, tokens, keys, connection strings, or credentials in code, tests, fixtures, or logs (example and template files with obvious placeholders excepted).
2. **Injection**: queries, shell commands, or templates built by string concatenation or interpolation of untrusted input instead of parameters or safe APIs.
3. **Authorization**: new or changed entry points without authentication and authorization checks consistent with the project's existing pattern.
4. **Data isolation**: scoped reads and writes without their isolation filter, or scope taken from client-supplied identifiers instead of trusted context.

## WARNING: should be fixed

5. Internal error details returned to clients.
6. Personal or sensitive data in log statements.
7. Debug output left in production code.
8. Blocking or unsafe concurrency patterns the project's guidelines forbid.
9. Missing input validation on new external inputs.

## INFO: review

10. New TODO, FIXME, or HACK comments.
11. Known-vulnerable dependencies, using the ecosystem's audit command when available.
12. Security-relevant configuration changes, which may need a `docs/SECURITY_AUDIT.md` guidance pass.

## Report

```
CRITICAL: <finding> - <path:line> - <suggested fix>
WARNING:  <finding> - <path:line> - <suggested fix>
INFO:     <finding>
Result: PASS (no CRITICAL findings) | FAIL
```

Never weaken a control, test, or check to make this report pass.
