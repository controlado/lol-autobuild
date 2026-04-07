# LCU Limitations and Fallback Strategy

LCU is an unofficial local API and can break at any time due to client updates.

## Known instability vectors

- Endpoint schema changes between client versions.
- Client lockfile availability race at startup.
- Session/champ-select state changes during apply operations.
- Local auth/session reset by Riot client updates.

## Project policy

- Treat LCU failures as operational warnings where possible.
- Champion autodetection is mandatory before recommendation generation.
- Fail fast on malformed requests, fail soft on transient LCU state errors.

## Fallback behavior

- `--dry-run` remains the safest mode for diagnostics, but still requires champion detection from LCU.
- If apply fails for one subsystem (items/runes/spells), continue processing others and emit warnings.
- Keep payloads and intended operations visible in logs (without secrets).

## Runtime prerequisite

- League Client must be running in champ select and your local player must have a champion selected.
- If `/lol-champ-select/v1/session` is unavailable, sync fails early with a clear detection error.

## Validation approach

- Integration tests with stubs/mocks for apply calls.
- No hard dependency on live LCU for CI.
