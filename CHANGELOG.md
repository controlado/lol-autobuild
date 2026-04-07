# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog principles and semantic versioning.

## [Unreleased]

### Changed

- Breaking: `SyncRequest` no longer accepts `ChampionID`; champion is now detected from LCU champ select.
- `lcu.enabled` now gates both champion detection and apply operations.

### Added

- `SyncResult.DetectedChampionID` to expose the champion resolved from LCU.
- LCU champion autodetection via lockfile + `GET /lol-champ-select/v1/session`.

- Initial private bootstrap for Go library-first project.
- Public sync service contract.
- Internal architecture ports and adapters scaffolding.
- Base CI and release automation.

### Removed

- CLI flag `--champion-id`.
