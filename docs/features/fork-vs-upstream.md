# Fork vs upstream

[rengwu/wayfinder-maps](https://github.com/rengwu/wayfinder-maps) stops where the decisions stop: a map is done when the way is clear, and the build happens somewhere else.
This fork lets a map carry on into implementation, so the whole journey lives in one graph.
A map with no implement tickets reads exactly as it does upstream.

![The same map in the upstream viewer (v0.2.3) and in this fork (v0.12.0), with callouts for implement stations, frontier badges, the start and goal, the split HUD, the ticket panel with its PR chip, the compass and the Pull from GitHub refresh button](../images/compare.png)

## What this fork adds

| Addition | What it gives you |
| --- | --- |
| [`implement` tickets](implement-tickets.md) | Shipped work lives on the map, with an optional PR link |
| [AFK or HITL frontier](afk-hitl.md) | Each ready ticket says whether an agent can take it |
| [Split `status`, lint, graph JSON](implement-tickets.md#status-lint-and-graph-json) | Decisions and implementation counted apart |
| [Start and goal in the viewer](start-and-goal.md) | A virtual start and a goal cloud holding the fog |
| [Compass](compass.md) | The map flows in any of 8 directions or radially |
| [Tickets placed by age](ticket-placement.md#age-nudges-it-forward) | Later tickets drift toward the goal without stretching the map |
| [Narrower columns on deep maps](ticket-placement.md#columns-narrow-with-depth) | Column gaps shrink with depth, so long chains fit the screen at a larger zoom |
| [Settings menu](settings.md) | A gear popover for viewer toggles with hover notes, saved across restarts; Freeze nodes stops the idle drift of the stars, Zoom with buttons swaps scroll-zoom for +/- buttons (handy in embeds), `?zoom=`, `?freeze=` and `?compass=` override the settings per embed, Update check turns the outdated notice on or off, gh account sets the default account for pulls |
| [`export` to a static site](export.md) | A map as a static, still interactive website to host, share, or embed in a page or presentation, no install needed to view it; the [demo](../demo.md) is one |
| [Narrow-screen layout](narrow-screens.md) | Wrapping top bar and a status bar that collapses to a corner, its fold tab always beside the compass |
| [`pull` in the binary](pull.md) | GitHub-issue maps mirrored into `.plan/`, fetched in batches of 50 tickets, with account choice and progress |
| [Refresh button](pull.md#refresh-from-the-viewer) | The same pull from the viewer |
| [Open from GitHub](open-from-github.md) | Find a map in a repo and open it from the splash, with account choice, no `pull` first |
| [Recent maps](recent-maps.md) | Maps opened from the CLI or the viewer are listed on the splash |
| [`install-skill`](install-skill.md) | Skills embedded in the binary, installed per harness |
| [Homebrew](../install.md#homebrew) | `brew install` and `brew upgrade` from a tap, with the binary aware of it |
| [`self-update`](self-update.md) | The binary upgrades itself, except under Homebrew, which hides and refuses it |
| [Outdated notice](update-notice.md) | A short stderr notice with the right update command when a newer release exists, and the latest version in the settings menu; cached for a day and easy to silence |
| [Logo](logo.md) | A small constellation as the favicon, the splash logo and the mark on the map |
| [Compass logo](compass-logo.md) | A compass in the docs header that turns the logo and the favicon to any of 8 directions or a radial one, remembered across pages and visits |
| [Light and dark mode](dark-mode.md) | A light star chart or a dark night sky behind a header toggle, with constellations and the odd shooting star |
| [`--version`](../install.md#check-the-version) | On `wayfinder-maps` and the Linux webview helper; the CLI adds how it was installed and where |

The [design record](https://github.com/RamsesKools/wayfinder-maps/blob/main/docs/implement-phase-design.md) has the full plan for the implement phase.

## Why these additions

Wayfinder exists because planning and deciding don't always fit in one session.
I think the same holds for implementation:

1. Some of the fog only clears once you start building.
   An implementation regularly turns up new questions that need a grilling, research or a prototype, and those belong on the same map rather than in a fresh plan.
2. I like tracking progress on the visual map, and an implementation too big for one session deserves that view as well.

So one map now tracks the decisions and the implementation, all the way until the work is done.
