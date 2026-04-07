# Coachless API Notes

## Strategy

- API-first data ingestion from `https://api.coachless.gg`.
- Avoid HTML scraping as primary path.

## Initial endpoints mapped

- `POST /api/Auth/refresh`
- `GET /api/ChampionWinprob/GetPatches`
- `POST /api/Rune/GetKeystoneData`
- `POST /api/ChampionWinprob/GetGlobalSummonerSpellStatistics`
- `POST /api/ChampionWinprob/GetGlobalItemStatistics`

## Auth behavior

- Some endpoints require bearer tokens.
- Token refresh endpoint accepts refresh token payload.

## Change risk

Coachless API can evolve without notice. Maintain contract tests and endpoint-level error handling.
