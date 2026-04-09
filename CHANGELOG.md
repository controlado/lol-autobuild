# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog principles and semantic versioning.

## [Unreleased]

### Changed

- Breaking: `SyncRequest` no longer accepts `ChampionID`; champion is now detected from LCU champ select.
- Breaking: `SyncRequest` no longer accepts manual `Role`; role is now detected from LCU champ select.
- `lcu.enabled` now gates champion+role detection and apply operations.
- Sync now fails fast in queues where role detection is unsupported (current allowlist: `queueId` 400/420/440).
- Added public watch orchestration: `Service.Watch(ctx, WatchRequest)`.
- `cmd/dev` now includes `watch` command with graceful shutdown on `CTRL+C`.

### Added

- `SyncResult.DetectedChampionID` to expose the champion resolved from LCU.
- `SyncResult.DetectedRole` and `SyncResult.DetectedQueueID`.
- LCU champion+role autodetection via lockfile + `GET /lol-champ-select/v1/session`.
- LCU websocket event stream support (`OnJsonApiEvent`) with reconnect loop.
- `LCUClient.WatchEvents(ctx, out)` and raw `LCUEvent` transport via channel.
- Watch configuration knobs: `watch.debounce_millis`, `watch.reconnect_delay_millis`.

- Initial private bootstrap for Go library-first project.
- Public sync service contract.
- Internal architecture ports and adapters scaffolding.
- Base CI and release automation.

### Removed

- CLI flag `--champion-id`.
- CLI flag `--role`.
