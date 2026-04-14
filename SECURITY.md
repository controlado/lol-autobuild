# Security

## Scope

This document covers secret handling, token handling, logging hygiene, and vulnerability reporting for `lol-autobuild`.

## Secret and token handling

- The project stores Coachless tokens in OS keyring through `internal/secrets`.
- The auth provider reads stored tokens first, then tries refresh flow when a refresh token exists.
- Manual fallback uses environment variables:
  - `COACHLESS_ACCESS_TOKEN` (required for manual path)
  - `COACHLESS_REFRESH_TOKEN` (optional)
  - `COACHLESS_ACCESS_TOKEN_EXP` (optional Unix timestamp)
- Keep lockfile paths and process arguments that include LCU auth material out of shared logs.
- Do not commit tokens, lockfiles, or local debug dumps that include credentials.

## Current auth implementation status

- Browser-assisted token capture source exists but is not implemented yet.
- Manual fallback source works and should be treated as sensitive input.
- Token validity checks use configured skew (`auth.token_skew_seconds`) to avoid near-expiry usage.

## Logging and diagnostics rules

- Never print raw access tokens, refresh tokens, or LCU auth values.
- Redact secrets before sharing logs or error reports.
- Keep error context, endpoint names, and status codes in reports so maintainers can reproduce failures without seeing credentials.

## Vulnerability reporting

If you find a security issue:

1. Report it privately to repository maintainers through your internal channel.
2. Include impact, attack path, affected components, and reproduction steps.
3. Avoid opening a public issue with exploit details or secrets.
4. Wait for maintainer guidance before broad disclosure.
