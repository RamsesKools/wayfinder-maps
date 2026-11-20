# Pull from GitHub

Upstream ships a `wayfinder-pull` script that mirrors a map kept as GitHub issues into `.plan/`.
Here it is a subcommand of the binary, with more on top:

- the `wayfinder:implement` and `wayfinder:afk` labels and the linked PRs are imported;
- `--account <gh-user>` pulls as another logged-in `gh` account, stored for later refreshes;
- a progress bar on a terminal, plain `pulled N/M items` lines in CI.

![The progress bar on a terminal](../images/pull-cli-tty.png)

[Maps kept as GitHub issues](../guide/github-issues.md) has the full reference.

## Refresh from the viewer

A Refresh from GitHub button in the top bar re-pulls the map and streams progress from the server (server-sent events) as a spinner and `N/M items`, then a done or error state with the message.
The account in use is shown next to the button.

![The pull button with the gh account it pulls as](../images/pull-account.png)

![Pull in progress: spinner and 5/13 items](../images/pull-progress.png)

![Pull finished](../images/pull-done.png)

![Pull failed: account not logged in](../images/pull-error.png)

To open a GitHub map without a `pull` first, see [Open a map from GitHub](open-from-github.md).
