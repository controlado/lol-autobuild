<p align="center">
  English | <a href="README.br.md">Português</a>
</p>

<div align="center">

# lol-autobuild

Set up League of Legends item sets and summoner spells from Coachless data.

<img width="1300" height="1000" alt="lol-autobuild local UI" src="https://github.com/user-attachments/assets/7d684e0c-a097-4939-895d-463a4408738e" />

</div>

## Download

[Download the latest release](https://github.com/controlado/lol-autobuild/releases/latest)

Pick the ZIP for your system, extract it, and run `lol-autobuild`.

## What it does

`lol-autobuild` connects to the League Client during champion select. It reads your champion and position, checks Coachless data, and prepares recommended item sets and summoner spells. Rune page apply is still pending.

## About Coachless

[Coachless](https://coachless.gg/) is a standout League of Legends analytics site. It uses Win Probability Added (WPA) to compare items with more context than raw win rate. Players get a smarter way to judge builds. [xPetu](https://x.com/xPetu) leads the project; players know him for high-level Shen play and math-based League analysis.

## First run

1. Open League of Legends.
2. Start `lol-autobuild`.
3. Use the local browser page that opens.
4. Log in to Coachless when the app asks.
5. Keep preview mode on until you trust the result.

The app runs on `127.0.0.1`, on your own computer.

## Basic commands

Open the local UI:

```bash
lol-autobuild
```

Preview one sync:

```bash
lol-autobuild sync --dry-run
```

Watch champion select and sync once during finalization:

```bash
lol-autobuild watch --dry-run
```

Use `--dry-run=false` only when you want the app to apply changes to the League Client.

Advanced commands, config, and limits live in [USAGE.md](USAGE.md).

## Disclaimer

`lol-autobuild` is an independent open source project. It has no affiliation with `coachless.gg`; it only reads Coachless data and local League Client APIs. Riot Games does not endorse or sponsor this repository, and it has no official connection to League of Legends. `League of Legends` and `Riot Games` are trademarks or registered trademarks of Riot Games, Inc.
