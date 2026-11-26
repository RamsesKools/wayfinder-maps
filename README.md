![The wayfinder star-map viewer on a release-notes map, laid out left to right from a start node to a goal cloud holding the fog, with teal implement stations, frontier badges, and a HUD with decision and implement counts](docs/images/demo.png)

# wayfinder-maps

A CLI and star-map viewer for [wayfinder](https://github.com/mattpocock/skills/tree/main/skills/engineering/wayfinder) maps, carried on from the first question into implementation.

- Plan and build work too big for one agent session.
- See the whole effort at a glance: decisions, dependencies, progress, and the fog that is left.
- Carry the map from the first question to the merged PRs.
- A CLI for agents, a star map for humans.

Wayfinder, created by [Matt Pocock](https://github.com/mattpocock), charts "an idea whose destination you can name but whose route you cannot yet see" as a map of tickets, resolved one per session, see [his doc](https://github.com/mattpocock/skills/blob/main/docs/engineering/wayfinder.md).
The CLI and viewer are by John Goh, and this fork extends both so a map carries on into implementation, see [fork vs upstream](docs/features/fork-vs-upstream.md).

Read the full story and try the [live demo](https://wayfinder.ramseskools.nl/) before installing anything.

## Install

The skills, as a Claude Code plugin:

```
/plugin marketplace add RamsesKools/wayfinder-maps
/plugin install wayfinder-maps@wayfinder-maps
```

The binary on macOS or Linux, then the matching skills for Claude Code, GitHub Copilot or Codex:

```sh
curl -fsSL https://raw.githubusercontent.com/RamsesKools/wayfinder-maps/main/scripts/install.sh | bash
wayfinder-maps install-skill
```

Or on macOS with Homebrew, which then also does the upgrading:

```sh
brew install RamsesKools/tap/wayfinder-maps
brew upgrade wayfinder-maps
```

Windows is not supported yet: I will add it when I need it myself, or when others [let me know](https://github.com/RamsesKools/wayfinder-maps/issues) they would like it.
Pinned releases, drop-in skills and updates are on the [install page](docs/install.md).

## Usage

1. Ask your agent to run `/wayfinder-maps` on a big, foggy effort. It charts the map under `.plan/<effort>/`.
2. Each later session claims a ticket from the frontier and resolves it.
3. Follow along from the terminal or the star map:

```sh
wayfinder-maps status .plan/<effort>   # resolved, in flight, and ready to claim
wayfinder-maps lint .plan/<effort>     # does the map still tell the truth?
wayfinder-maps app .plan/<effort>      # the star map in a native window (serve for a browser)
wayfinder-maps export .plan/<effort> site   # the map as a static website to host, share or embed
```

The [guide](docs/guide/usage.md) covers every command, the map format and maps kept as GitHub issues.

## Contributing

- Vote on [issues](https://github.com/RamsesKools/wayfinder-maps/issues) with a thumbs-up reaction, the most wanted go first.
- Raise ideas and ask questions in [Discussions](https://github.com/RamsesKools/wayfinder-maps/discussions), under Ideas or Q&A.

## Development

```sh
go build ./... && go vet ./... && go test ./...
node --test cmd/wayfinder-maps/webtests/*.test.mjs   # the viewer
uv run mkdocs build --strict                         # the docs site
```

More on [building from source](docs/install.md#build-from-source).

## License

MIT, see [LICENSE](https://github.com/RamsesKools/wayfinder-maps/blob/main/LICENSE).
The CLI and viewer are by John Goh, the wayfinder method and skills by Matt Pocock (MIT, see [skills/LICENSE](https://github.com/RamsesKools/wayfinder-maps/blob/main/skills/LICENSE)).
This is a fork of [rengwu/wayfinder-maps](https://github.com/rengwu/wayfinder-maps) and is not merged back upstream.
