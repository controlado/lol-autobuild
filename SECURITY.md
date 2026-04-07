# Security

## Secret handling

- Default secret backend: OS keyring.
- Store only what is needed for token refresh flow.
- Never log raw tokens.

## Auth model

- Primary: browser-assisted token acquisition.
- Fallback: manual source (explicit opt-in).

## Risk notes

- LCU integration is inherently unstable and unofficial.
- Coachless API contracts can change and must be monitored.
- Browser automation is sensitive to anti-bot and login flow changes.

## Operational guidance

- Redact sensitive values in logs and crash reports.
- Keep token lifetime checks conservative (clock skew allowance).
- Fail closed on invalid token parsing.

## Disclosure

This project is private and intended for controlled internal usage in this phase.
