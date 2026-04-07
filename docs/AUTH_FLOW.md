# Auth Flow (Dual Mode)

The project supports two token acquisition modes:

1. Browser-assisted source (default path)
2. Manual fallback source

## Resolution order

1. Read persisted token pair from secret store.
2. If access token is valid, use it.
3. If expired but refresh token exists, refresh via Coachless API.
4. If refresh fails and auto mode is enabled, try browser-assisted acquire.
5. If auto fails and manual fallback is enabled, read manual source.

## Secret persistence

- Default backend: OS keyring.
- Stored data: token pair and expiry metadata.

## Operational notes

- Never print raw token values.
- Maintain token skew to avoid near-expiry race conditions.
- Manual fallback is intended for recovery and development, not the ideal UX path.
