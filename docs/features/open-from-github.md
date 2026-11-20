# Open a map from GitHub

A map kept as [GitHub issues](../guide/github-issues.md) opens straight from the splash, with no `pull` first.
It needs the [GitHub CLI](https://cli.github.com), logged in.

1. The splash shows the `gh` account in use, with a dropdown (the viewer's own, so it looks the same everywhere) to switch when several are logged in.
   Without `gh`, or without a login, it shows a hint (install it, or run `gh auth login`) and dims and disables the button.
2. Paste `owner/repo` or a GitHub URL into the repo box, or pick from its dropdown: the 10 repos you were most recently active in (pushes, PRs, issues, comments; stars do not count), plus repos you opened from GitHub before.
3. The repo is scanned for maps, which are issues with sub-issues where a child carries a `wayfinder:<type>` label.
   Each map shows its percent completed, and closed maps are dimmed.
   The scan reads the 100 most recently updated issues of the repo.
4. `Open from GitHub` pulls the map with the usual progress into `<user cache dir>/wayfinder-maps/github/<owner>-<repo>/<slug>`, then opens it and lists it under [Recent](recent-maps.md).
   A map already in the cache opens at once instead, and the viewer starts the same refresh as the `Refresh from GitHub` button, so you see the spinner and `N/M items` in the top bar while it updates.

![The repo box with its list of recently active repos open, and a chevron on the right of the box](../images/github-open-repos.png){ width="420" }

![A map opened from the cache straight away, with the top bar showing the Refresh from GitHub button, the gh account and a spinner at 3/11 items while the refresh runs](../images/github-open-cached-refresh.png){ width="720" }

![The splash with the gh account, a pasted repo URL and the map dropdown showing three maps with their percent completed, the closed one dimmed](../images/github-open-pick-map.png){ width="420" }

![The splash without a gh login: a hint to run gh auth login, and the repo box, map dropdown and Open from GitHub button dimmed](../images/github-open-not-logged-in.png){ width="420" }

The copy is a read-only mirror: nothing is written back to GitHub, and the `Refresh from GitHub` button refreshes it.
Cached maps carry a `clean up` button in Recent, next to the `x` that only forgets the entry.

![The Recent list with a GitHub map hovered, showing its clean up and forget buttons](../images/github-open-recents.png){ width="420" }

![The Recent list scrolled to its bottom, showing it scrolls when long](../images/github-open-recents-scrolled.png){ width="420" }

![The map opened from GitHub, with the repo name in the top bar and the pull button next to the account](../images/github-open-map.png)
