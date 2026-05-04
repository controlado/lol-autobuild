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

## Auth flow

- The auth provider reads stored tokens first.
- If a refresh token exists, the provider tries the Coachless refresh flow before asking for a new login.
- Browser-assisted token capture opens Coachless login and stores tokens from the login response when `auth.auto_enabled` is true.
- Manual fallback reads environment variables when `auth.manual_fallback_enabled` is true.
- Token validity checks use configured skew (`auth.token_skew_seconds`) to avoid near-expiry usage.

## Logging and diagnostics rules

- Never print raw access tokens, refresh tokens, or LCU auth values.
- Redact secrets before sharing logs or error reports.
- Keep error context, endpoint names, and status codes in reports so maintainers can reproduce failures without seeing credentials.

## Vulnerability reporting

If you find a security issue:

1. Report it privately to repository maintainers. Use GitHub private vulnerability reporting if the repo enables it; otherwise use a private maintainer contact.
2. Include impact, attack path, affected components, and reproduction steps.
3. Avoid opening a public issue with exploit details or secrets.
4. Wait for maintainer guidance before broad disclosure.
