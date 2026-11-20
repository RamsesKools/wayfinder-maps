# Agent instructions

## Checks

- `go build ./... && go vet ./... && go test ./...`
- `node --test cmd/wayfinder-maps/webtests/*.test.mjs` for the viewer.
- `uv run mkdocs build --strict` for the docs site.
- Verify viewer changes in a headless browser (see the `verify` skill).

## Viewer settings

To add a setting to the gear popover:

- `settings.go`: add the field to `settings` and `settingsPatch`, merge it in `apply`, and copy it in `loadSettings`.
- `server_test.go`: extend `TestSettingsEndpoint` (default, round-trip, restart, partial update, invalid body).
- `index.html`: copy the `.setrow` in `#setpop` (name, `?` button, `.tip` note, switch) with new ids.
- `js/state.js`, `js/settings.js`, `js/main.js`: add the `S` field, save it with a partial `POST /api/settings`, and apply it from the boot read.
- Docs: add a row to the table in `docs/features/settings.md` and, if it needs more than a line, its own page.

## Git and PRs

- Push to the fork (`origin`, RamsesKools) and open PRs there, not on `rengwu/wayfinder-maps`, until merging back upstream has been discussed.
- Link upstream issues without closing keywords (`Refs rengwu/wayfinder-maps#N`) so merging in the fork does not close them.
- Add visual proof to every PR that touches the viewer: screenshots of the change working, embedded in the PR body.

## Docs

The docs site is MkDocs Material (`mkdocs.yml`, pages in `docs/`, images in `docs/images/`), deployed to GitHub Pages by the `release` workflow.
`docs/index.md` includes `README.md` via snippets, so README links must work on GitHub and on the site: `docs/...` paths for docs pages and images, absolute GitHub URLs for anything else in the repo.

- A PR that adds or changes a user-facing feature updates its page under `docs/features/` with visuals (screenshots or short diagrams in `docs/images/`), not only prose, and the table in `docs/features/index.md` (fork vs upstream).
- Change the README only when the overview, install or basic usage changes.
- Design notes (`docs/starmap-design.md`, `docs/implement-phase-design.md`) stay out of the site.

## Releases

- Bump the version in `.claude-plugin/plugin.json` in every PR, the only version source; the `version-bump` CI job fails PRs that do not raise it.
- After merging, run the `release` workflow from `main` (Actions tab or `gh workflow run release --ref main`); it tags `v<version>` and publishes the GitHub release.
- Re-run a failed release from its existing `vX.Y.Z` tag: `gh workflow run release --ref vX.Y.Z`.
