# Advanced Usage

Technical reference for `lol-autobuild`.

For the short user guide, read [README.md](README.md). Portuguese version: [README.br.md](README.br.md).

## What it does

`lol-autobuild` runs a sync cycle that:

- Detects your current champion and position from the local LCU champ select session.
- Pulls Coachless patch, keystone, summoner spell, and item stats.
- Builds recommendations.
- Applies supported changes in LCU. Set `--dry-run=true` or `sync.dry_run: true` to preview the plan.

## Capability matrix

| Capability | Status | Notes |
| --- | --- | --- |
| Champion and position detection from LCU | Implemented | Detection runs against `/lol-champ-select/v1/session`. |
| Position detection queues | Implemented | Supported queue IDs: `400`, `420`, `440`, `3110`. |
| Coachless API ingestion | Implemented | Uses API-first flow from `https://api.coachless.gg`. |
| Item set apply | Implemented | Upserts a managed item set in LCU. |
| Summoner spells apply | Implemented | Applies two spells and preserves the current Flash slot when possible. |
| Rune page apply | Pending | Current adapter returns not configured. |
| Watch mode (`watch`) | Implemented | Syncs once per champ select when the session timer enters `FINALIZATION`. |
| Local settings UI | Implemented | Opens a local browser page served from `127.0.0.1`. |
| Browser-assisted auth capture | Implemented | Opens Coachless login and stores tokens from the login response. |
| Manual auth fallback via environment | Implemented | Reads `COACHLESS_ACCESS_TOKEN`, optional refresh and exp fields from process env. |

## Prerequisites

- League Client running, with champ select available.
- A valid config file, starting from `config.example.yaml` when you need custom settings.
- `lcu.enabled: true` when you want detection and LCU apply operations.
- Coachless token access through one of these paths:
  - Token pair already stored in OS keyring.
  - Browser-assisted Coachless login.
  - Environment fallback with optional preload from `env_file.path`.

Environment variables for manual auth:

```bash
COACHLESS_ACCESS_TOKEN=...
COACHLESS_REFRESH_TOKEN=...
COACHLESS_ACCESS_TOKEN_EXP=...
```

`COACHLESS_REFRESH_TOKEN` and `COACHLESS_ACCESS_TOKEN_EXP` are optional.

## Local UI

The default command starts a local web server on `127.0.0.1`, opens your browser, and keeps running until you press `CTRL+C`.

```bash
lol-autobuild
```

Run the UI with a specific config file:

```bash
lol-autobuild ui --config ./config.example.yaml
```

The UI lets you:

- Change what sync updates: items, runes, and summoner spells.
- Switch between live apply mode and preview mode.
- Run one sync.
- Start or stop the watcher.
- Check the current League Client connection state.

The UI uses live apply mode when a config omits `sync.dry_run`. Set `sync.dry_run: true` for preview mode.

The API uses a per-run token in local URLs. The server does not listen on public network interfaces.

## Command reference

### `lol-autobuild`

Opens the local settings UI.

### `lol-autobuild ui`

Opens the local settings UI.

Flags:

- `--config string` (default `"config.yaml"`)

### `lol-autobuild sync`

Runs one synchronization cycle.

```bash
lol-autobuild sync --config ./config.example.yaml --dry-run
```

Flags:

- `--apply-items` (default `true`)
- `--apply-runes` (default `true`)
- `--apply-spells` (default `true`)
- `--config string` (default `"config.yaml"`)
- `--dry-run` (default `true`)
- `--patch string` (empty = latest patch from Coachless)

### `lol-autobuild watch`

Watches LCU champ select events. It runs one synchronization cycle per champ select when `/lol-champ-select/v1/session` reports `data.timer.phase == "FINALIZATION"`.

```bash
lol-autobuild watch --config ./config.example.yaml --dry-run
```

Flags:

- `--apply-items` (default `true`)
- `--apply-runes` (default `true`)
- `--apply-spells` (default `true`)
- `--config string` (default `"config.yaml"`)
- `--dry-run` (default `true`)
- `--patch string` (empty = latest patch from Coachless)

`watch` waits for champ select finalization before it syncs. It does not run a sync cycle at startup.

CLI `sync` and `watch` default to `--dry-run=true`. Pass `--dry-run=false` to apply LCU changes from the CLI.

## Config reference

`config.example.yaml` follows this structure:

| Key | Type | Default in code | Purpose |
| --- | --- | --- | --- |
| `log_level` | string | `info` | Global log level. |
| `coachless.api_base_url` | string | `https://api.coachless.gg` | Coachless API base URL. |
| `coachless.timeout_seconds` | int | `20` | Coachless request timeout. |
| `auth.auto_enabled` | bool | `true` | Enables browser-assisted Coachless token capture. |
| `auth.manual_fallback_enabled` | bool | `true` | Enables env-based fallback source. |
| `auth.token_skew_seconds` | int | `30` | Token validity skew before expiry. |
| `env_file.path` | string | `""` | Optional path to `.env` file loaded before bootstrap. |
| `secrets.service_name` | string | `lol-autobuild` | OS keyring service name. |
| `recommendation.min_occurrence` | int | `1000` | Minimum occurrence count for recommendation candidates. |
| `recommendation.top_items` | int | `6` | Max recommended item count per block. `0` disables the limit. |
| `recommendation.top_spells` | int | `2` | Max recommended spell count. |
| `lcu.enabled` | bool | `false` | Enables LCU detection and apply paths. |
| `lcu.lockfile_path` | string | `""` | Optional lockfile fallback path. |
| `sync.patch` | string | `""` | Patch label used by the local UI. Empty means latest. |
| `sync.apply_items` | bool | `true` | Local UI setting for item set apply. |
| `sync.apply_runes` | bool | `true` | Local UI setting for rune page apply. |
| `sync.apply_spells` | bool | `true` | Local UI setting for summoner spell apply. |
| `sync.keep_flash` | bool | `true` | Keep your current Flash slot when applying summoner spells. |
| `sync.dry_run` | bool | `false` | Local UI mode. Use `true` for preview. |
| `watch.debounce_millis` | int | `500` | Debounce window after finalization events. |
| `watch.reconnect_delay_millis` | int | `1000` | Delay before websocket reconnect attempts. |

When `env_file.path` is set, the CLI loads that file before service bootstrap. Relative paths resolve from the config file directory. Existing process environment variables keep precedence over values from the file. Startup fails if the configured file does not exist.

LCU connection discovery tries League process args first (`--app-port`, `--remoting-auth-token`, optional `--app-protocol`), then falls back to `lcu.lockfile_path`.

## Operational limits

- Sync requires a working LCU connection, even in dry-run mode, because champion and position detection runs first.
- Sync fails early when champ select is unavailable, champion is not selected, or the queue is not in the supported position-detection list.
- When apply fails for one subsystem, the service keeps running the others and reports warnings in `SyncResult`.
- Watch mode ignores startup and early champ select phases.
- Watch mode only reacts to champ select session `Create` and `Update` events from `/lol-champ-select/v1/session` when `data.timer.phase == "FINALIZATION"`.
- Watch mode attempts one sync per champ select. A session `Delete` or a new non-finalized `Create` event resets that lock.
- If the finalization sync fails, watch mode waits for the next champ select before it tries again.
- The local UI uses the `sync` config section. CLI flags still control `sync` and `watch`.
- Free Coachless tokens use the latest non-Premium patch when the patch setting is blank. Requesting the newest Premium patch returns an error.
- Rune page apply is not implemented yet.
- Browser-assisted auth capture watches the Coachless login response and stores the token pair.

## Next work

- Implement LCU rune page apply path.
- Expand queue coverage for position detection.
- Add clearer diagnostics around auth and LCU failures.
