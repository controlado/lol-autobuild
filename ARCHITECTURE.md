# Architecture

## Overview

`lol-autobuild` is a library-first Go module with a thin CLI for development and manual verification.

Data flow:

1. Detect local champion from LCU champ select session.
2. Resolve auth token via `TokenProvider`.
3. Query Coachless API endpoints.
4. Build recommendation model.
5. Apply actions through `LCUClient` (items/runes/spells) or dry-run.
6. Return structured sync result and warnings.

## Layout

- `pkg/lolautobuild`: public service interface and sync orchestration entrypoint.
- `internal/ports`: domain contracts shared across internal adapters.
- `internal/config`: YAML config load + validation.
- `internal/coachless`: Coachless API client.
- `internal/auth`: token orchestration (auto + manual fallback).
- `internal/secrets`: OS keyring-backed secret store.
- `internal/recommend`: recommendation policy/mapping.
- `internal/lcu`: LCU adapter interface implementations.
- `cmd/dev`: development CLI.

## Core contracts

- Public:
  - `Service.Sync(ctx, SyncRequest) (SyncResult, error)`
- Internal ports:
  - `CoachlessClient`
  - `TokenProvider`
  - `SecretStore`
  - `LCUClient`
  - `RecommendationEngine`

`SyncRequest` no longer accepts manual champion ID. Champion selection is mandatory via LCU detection.

## Authentication strategy

Dual mode:

1. Browser-assisted auto source (default path).
2. Manual fallback source (environment/config-driven).

Token persistence is handled via OS keyring abstraction.

## Reliability principles

- API-first ingestion.
- Context-aware operations with timeouts.
- Retry/backoff support in external calls.
- LCU volatility explicitly handled via warnings and non-fatal flows where possible.
