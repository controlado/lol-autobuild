<div align="center">

# lol-autobuild

Automate League of Legends setup from Coachless data.

[Quick Start](#quick-start) • [Command Reference](#command-reference) • [Config Reference](#config-reference) • [Security](./SECURITY.md) • [Changelog](./CHANGELOG.md)

</div>

This project was originally developed in a private repository and is now open source.

## Disclaimer

`lol-autobuild` is an independent open source project. It has no affiliation with `coachless.gg`; it only reads Coachless data and local League Client APIs. Riot Games does not endorse or sponsor this repository, and it has no official connection to League of Legends. `League of Legends` and `Riot Games` are trademarks or registered trademarks of Riot Games, Inc.

## What it does

`lol-autobuild` runs a sync cycle that:

- Detects your current champion and Position from the local LCU champ select session.
- Pulls Coachless patch, keystone, summoner spell, and item stats.
- Builds recommendations.
- Applies supported changes in LCU, or reports the plan when `--dry-run=true`.

## Capability matrix

| Capability | Status | Notes |
| --- | --- | --- |
| Champion and Position detection from LCU | Implemented | Detection runs against `/lol-champ-select/v1/session`. |
| Position detection queues | Implemented | Supported queue IDs: `400`, `420`, `440`, `3110`. |
| Coachless API ingestion | Implemented | Uses API-first flow from `https://api.coachless.gg`. |
| Item set apply | Implemented | Upserts a managed item set in LCU. |
| Summoner spells apply | Implemented | Applies two spells and preserves the current Flash slot when possible. |
| Rune page apply | Pending | Current adapter returns not configured. |
| Watch mode (`dev watch`) | Implemented | Syncs once per champ select when the session timer enters `FINALIZATION`. |
| Browser-assisted auth capture | Pending | Browser source exists but is not implemented yet. |
| Manual auth fallback via environment | Implemented | Reads `COACHLESS_ACCESS_TOKEN`, optional refresh and exp fields from process env. |

## Prerequisites

- Go `1.24+`.
- League Client running, with champ select available.
- A valid config file (start from `config.example.yaml`).
- `lcu.enabled: true` when you want detection and LCU apply operations.
- Coachless token access through one of these paths:
  - Token pair already persisted in OS keyring.
  - Environment fallback (`COACHLESS_ACCESS_TOKEN`, optional `COACHLESS_REFRESH_TOKEN`, optional `COACHLESS_ACCESS_TOKEN_EXP`), with optional preload from `env_file.path`.

## Quick start

Run one sync cycle in dry-run mode:

```bash
go run ./cmd/dev sync --config ./config.example.yaml --dry-run
```

Run watch mode:

```bash
go run ./cmd/dev watch --config ./config.example.yaml --dry-run
```

`dev watch` waits for champ select finalization before it syncs. It does not run a sync cycle at startup.

`--dry-run` defaults to `true` for both commands. Use `--dry-run=false` only when you want live LCU changes.

## Command reference

### `dev sync`

Runs one synchronization cycle.

Flags:

- `--apply-items` (default `true`)
- `--apply-runes` (default `true`)
- `--apply-spells` (default `true`)
- `--config string` (default `"config.example.yaml"`)
- `--dry-run` (default `true`)
- `--patch string` (empty = latest patch from Coachless)

### `dev watch`

Watches LCU champ select events. It runs one synchronization cycle per champ select when `/lol-champ-select/v1/session` reports `data.timer.phase == "FINALIZATION"`.

Flags:

- `--apply-items` (default `true`)
- `--apply-runes` (default `true`)
- `--apply-spells` (default `true`)
- `--config string` (default `"config.example.yaml"`)
- `--dry-run` (default `true`)
- `--patch string` (empty = latest patch from Coachless)

## Config reference

`config.example.yaml` follows this structure:

| Key | Type | Default in code | Purpose |
| --- | --- | --- | --- |
| `log_level` | string | `info` | Global log level. |
| `coachless.api_base_url` | string | `https://api.coachless.gg` | Coachless API base URL. |
| `coachless.timeout_seconds` | int | `20` | Coachless request timeout. |
| `auth.auto_enabled` | bool | `true` | Enables browser-assisted source (pending implementation). |
| `auth.manual_fallback_enabled` | bool | `true` | Enables env-based fallback source. |
| `auth.token_skew_seconds` | int | `30` | Token validity skew before expiry. |
| `env_file.path` | string | `""` | Optional path to `.env` file loaded before bootstrap. |
| `secrets.service_name` | string | `lol-autobuild` | OS keyring service name. |
| `recommendation.min_occurrence` | int | `100` | Minimum sample threshold for recommendations. |
| `recommendation.top_items` | int | `6` | Max recommended item count. |
| `recommendation.top_spells` | int | `2` | Max recommended spell count. |
| `lcu.enabled` | bool | `false` | Enables LCU detection and apply paths. |
| `lcu.lockfile_path` | string | `""` | Optional lockfile fallback path. |
| `watch.debounce_millis` | int | `500` | Debounce window after finalization events. |
| `watch.reconnect_delay_millis` | int | `1000` | Delay before websocket reconnect attempts. |

When `env_file.path` is set, the CLI loads that file before service bootstrap. Relative paths are resolved from the config file directory. Existing process environment variables keep precedence over values from the file. Startup fails if the configured file does not exist.

LCU connection discovery tries League process args first (`--app-port`, `--remoting-auth-token`, optional `--app-protocol`), then falls back to `lcu.lockfile_path`.

## Operational limits

- Sync requires a working LCU connection, even in dry-run mode, because champion and position detection always runs first.
- Sync fails early when champ select is unavailable, champion is not selected, or the queue is not in the supported position-detection list.
- When apply fails for one subsystem, the service keeps running the others and reports warnings in `SyncResult`.
- Watch mode ignores startup and early champ select phases.
- Watch mode only reacts to champ select session `Create` and `Update` events from `/lol-champ-select/v1/session` when `data.timer.phase == "FINALIZATION"`.
- Watch mode attempts one sync per champ select. A session `Delete` or a new non-finalized `Create` event resets that lock.
- If the finalization sync fails, watch mode waits for the next champ select before it tries again.
- Rune page apply is not implemented yet.
- Browser-assisted auth capture is not implemented yet.

## Next work

- Implement LCU rune page apply path.
- Implement browser-assisted auth capture flow.
- Expand queue coverage for position detection.
- Add richer operational diagnostics around auth and LCU failures.
