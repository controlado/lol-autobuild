# Architecture

## Overview

`lol-autobuild` is a library-first Go module with a thin CLI for development and manual verification.

Data flow:

1. Resolve auth token via `TokenProvider`.
2. Query Coachless API endpoints.
3. Build recommendation model.
4. Apply actions through `LCUClient` (items/runes/spells) or dry-run.
5. Return structured sync result and warnings.

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
