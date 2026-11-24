# Configuration

wayfinder-maps keeps its preferences in one settings file, and reads a few environment variables for overrides and tooling.

## settings.json

`settings.json` lives in the `wayfinder-maps` folder of your user config directory (on macOS, `~/Library/Application Support/wayfinder-maps/`), next to `recents.json` and `github-repos.json`.
The viewer's [settings menu](features/settings.md) edits it, and you can edit it by hand: a file with just one key is fine, missing keys keep their defaults.
A file that is not valid JSON, or holds an invalid value, falls back to the defaults.

```json
{
  "direction": "E",
  "freezeNodes": false,
  "updateCheck": true,
  "ghAccount": "",
  "zoomButtons": false
}
```

| Key | Default | What it does |
| --- | --- | --- |
| `direction` | `"E"` | Flow direction of the map: a [compass](features/compass.md) point (`N`, `NE`, `E`, `SE`, `S`, `SW`, `W`, `NW`) or `"none"` for the radial layout. |
| `freezeNodes` | `false` | Stops the idle drift of the stars, see [Freeze nodes](features/settings.md#freeze-nodes). |
| `zoomButtons` | `false` | Zooms with +/- buttons instead of the wheel, see [Zoom with buttons](features/settings.md#zoom-with-buttons). |
| `updateCheck` | `true` | Looks for a newer release and prints the [outdated notice](features/update-notice.md). `false` turns off both. |
| `ghAccount` | `""` | The `gh` account `pull` and Open from GitHub use by default. Empty means the active `gh` account. A GitHub login, see [Choosing the gh account](guide/github-issues.md#choosing-the-gh-account). |

## Environment variables

| Variable | What it does | Also a setting |
| --- | --- | --- |
| `WAYFINDER_GH_ACCOUNT` | Default `gh` account for `pull` and Open from GitHub. | `ghAccount`, the env var wins |
| `GH_TOKEN`, `GITHUB_TOKEN` | Token `pull` uses when no `gh` account is chosen, read by `gh` as well. | No, it is a secret |
| `PORT` | Port `serve` listens on, `7777` by default. | No |
| `WAYFINDER_RELEASES_URL` | Release host for `install.sh`, `self-update` and the update check, for mirrors and tests. | No |
| `WAYFINDER_API_URL` | GitHub API host used by `install.sh` and `self-update` to look up the release commit. | No |
| `WAYFINDER_DEV` | Path to `cmd/wayfinder-maps/web`, to serve the viewer from disk with no caching while developing it. | No |
| `CI` | When set, the outdated notice is not printed. | No |

Where both exist, a more specific choice beats a broader one.
For the `gh` account that is `--account`, then the account stored in the map's `.source.json`, then `WAYFINDER_GH_ACCOUNT`, then `ghAccount`, then the active `gh` account.

## Other files

| File | Where | What it holds |
| --- | --- | --- |
| `recents.json`, `github-repos.json` | config dir, `wayfinder-maps` folder | [Recent maps](features/recent-maps.md) and the repos offered when [opening from GitHub](features/open-from-github.md). |
| `latest-release.json` | cache dir, `wayfinder-maps` folder | The cached latest release for the [outdated notice](features/update-notice.md). Safe to delete. |
| `.install.json` | next to the binary | What `install.sh` installed, read by [`self-update`](features/self-update.md). |
| `.source.json` | in a pulled map | Where a [pulled map](features/pull.md) came from, including its `gh` account. |
