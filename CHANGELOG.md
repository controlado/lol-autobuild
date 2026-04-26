# Changelog

All notable changes to this project appear in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Added champion and position autodetection through LCU champ select session reads.
- Added detected selection fields to `SyncResult` (`DetectedChampionID`, `DetectedRole`, `DetectedQueueID`).
- Added item set apply flow with managed set upsert in LCU.
- Added staged item set block apply in LCU with ordered block support.
- Added summoner spell apply flow with Flash slot preservation behavior.
- Added watch orchestration (`Service.Watch`) and `dev watch` command.
- Added LCU websocket event stream support with reconnect flow.
- Added browser-assisted Coachless token capture.

### Changed

- `SyncRequest` no longer accepts manual champion or position input. Sync now relies on LCU detection.
- `ApplyItemSetRequest` now accepts staged `Blocks` and no longer accepts legacy flat `ItemIDs`.
- `lcu.enabled` now gates champion and position detection plus apply operations.
- LCU connection discovery now checks League process arguments first, then falls back to `lcu.lockfile_path`.
- Position detection now supports queue IDs `400`, `420`, `440`, and `3110`.
- Sync item recommendations now build ordered staged blocks (`Starter`, `1st Item`, `2nd Item`, `Boots`, `3rd Item`, `4th+ Item`) with per-block filtering.
- `dev sync` and `dev watch` continue to default to `--dry-run=true`.
- `dev watch` no longer syncs at startup. It syncs once per champ select when the session timer enters `FINALIZATION`.

### Removed

- Removed `--champion-id` CLI flag.
- Removed `--role` CLI flag.

### Pending

- Rune page apply is still pending in the LCU adapter.
