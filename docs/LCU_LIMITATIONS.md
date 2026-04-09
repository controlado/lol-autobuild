# LCU Limitations and Fallback Strategy

LCU is an unofficial local API and can break at any time due to client updates.

## Known instability vectors

- Endpoint schema changes between client versions.
- Client lockfile availability race at startup.
- Session/champ-select state changes during apply operations.
- Local auth/session reset by Riot client updates.
- WebSocket disconnects during client restarts/patching.

## Project policy

- Treat LCU failures as operational warnings where possible.
- Champion and role autodetection are mandatory before recommendation generation.
- Fail fast on malformed requests, fail soft on transient LCU state errors.

## Fallback behavior

- `--dry-run` remains the safest mode for diagnostics, but still requires champion detection from LCU.
- If apply fails for one subsystem (items/runes/spells), continue processing others and emit warnings.
- Keep payloads and intended operations visible in logs (without secrets).
- `watch` retries websocket connection using configured reconnect delay.

## Runtime prerequisite

- League Client must be running in champ select and your local player must have champion + assigned role available.
- Role autodetection is currently supported only for Summoner's Rift Draft/Ranked queues (`queueId` 400, 420, 440).
- If `/lol-champ-select/v1/session` is unavailable, sync fails early with a clear detection error.

## Validation approach

- Integration tests with stubs/mocks for apply calls.
- No hard dependency on live LCU for CI.
