<p align="center">
  English | <a href="README.br.md">Português</a>
</p>

<div align="center">

# lol-autobuild

Set up League of Legends rune pages, item sets, and summoner spells from Coachless data.

https://github.com/user-attachments/assets/de2f0f5b-5535-4d4f-b53e-b0a1abefd087

[Watch the demo video](docs/assets/demo/lol-autobuild-demo.mp4)

</div>

## Download

[Download the latest release](https://github.com/controlado/lol-autobuild/releases/latest)

Pick the ZIP for your system, extract it, and run `lol-autobuild`.

## What it does

`lol-autobuild` connects to the League Client during champion select. It reads your champion and position, checks Coachless data, and applies recommended rune pages, item sets, and summoner spells.

## About Coachless

[xPetu](https://x.com/xPetu) leads [Coachless](https://coachless.gg/), a League of Legends analytics site. It uses Win Probability Added (WPA) to compare items with more context than raw win rate. `lol-autobuild` uses its rune, item, and summoner spell data during champion select.

## First run

1. Open League of Legends.
2. Start `lol-autobuild`.
3. Use the local browser page that opens.
4. Log in to Coachless when the app asks.
5. The UI opens in live apply mode. Turn on preview mode for a dry run.

The app runs on `127.0.0.1`, on your own computer.

## Basic commands

Open the local UI:

```bash
lol-autobuild
```

Preview one CLI sync:

```bash
lol-autobuild sync --dry-run
```

Watch champion select in CLI preview mode:

```bash
lol-autobuild watch --dry-run
```

CLI commands read `sync.dry_run` from config. Pass `--dry-run` to preview or `--dry-run=false` to apply changes to the League Client.

Advanced commands, config, and limits live in [USAGE.md](USAGE.md).

## Thanks

Thanks to Riot Games for keeping the local League Client API accessible enough for tools like this to exist.

Thanks to [xPetu](https://x.com/xPetu) for building Coachless and keeping its unofficial API available to the community.

## Disclaimer

`lol-autobuild` is an independent open source project. It has no affiliation with `coachless.gg`; it only reads Coachless data and local League Client APIs. Riot Games does not endorse or sponsor this repository, and it has no official connection to League of Legends. `League of Legends` and `Riot Games` are trademarks or registered trademarks of Riot Games, Inc.
