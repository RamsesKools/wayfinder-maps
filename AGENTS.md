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
- Docs: add a section for it to `docs/features/settings.md`.

## Static export

`wayfinder-maps export` (and the docs demo, rebuilt by every `mkdocs build`) has no server behind it, so the viewer runs with `S.static` set.

- Mark every element in `index.html` that needs the server with `data-server-only`: one CSS rule hides it in static mode, so never hide things by id in CSS or JS.
- Make the JS behind it skip its `/api` call when `S.static` is set.
- A setting that still makes sense without a server is kept in `localStorage` via `loadPrefs` and `savePrefs` in `js/static.js`; one that needs the server gets `data-server-only` on its `.setrow`.
- `webtests/index.test.mjs` pins the marked elements: add yours there.
- Check a viewer change against the export too: serve a built `site/demo-fullscreen/` with a plain file server and confirm no `/api` requests, console errors or dead buttons.

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
- Every push to `main` runs the `release` workflow, which tags `v<version>` and publishes the GitHub release; it fails on a push whose version is already tagged on another commit, so bump it. Run it by hand with `gh workflow run release --ref main`.
- Re-run a failed release from its existing `vX.Y.Z` tag: `gh workflow run release --ref vX.Y.Z`.
