# Agent instructions

## Checks

- `go build ./... && go vet ./... && go test ./...`
- `node --test cmd/wayfinder-maps/webtests/*.test.mjs` for the viewer.
- Verify viewer changes in a headless browser (see the `verify` skill).

## Git and PRs

- Push to the fork (`origin`, RamsesKools) and open PRs there, not on `rengwu/wayfinder-maps`, until merging back upstream has been discussed.
- Link upstream issues without closing keywords (`Refs rengwu/wayfinder-maps#N`) so merging in the fork does not close them.
- Add visual proof to every PR that touches the viewer: screenshots of the change working, embedded in the PR body.

## Docs

- Document user-facing features in the README with visuals (screenshots or short diagrams in `docs/images/`), not only prose.

## Releases

- Bump the version in `.claude-plugin/plugin.json` in the PR, the only version source.
- After merging, run the `release` workflow from `main` (Actions tab or `gh workflow run release --ref main`); it tags `v<version>` and publishes the GitHub release.
- Re-run a failed release from its existing `vX.Y.Z` tag: `gh workflow run release --ref vX.Y.Z`.
