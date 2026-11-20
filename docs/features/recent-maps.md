# Recent maps

Every map you open with `serve`, `app` or the viewer is remembered on this machine, so the `Maps` button and the splash list it next time.

- A map with a `map.md` is one entry, any other folder is a project entry that opens its map list.
- A path that fails to load, or a port that is already taken, records nothing.
- The list holds 20 entries, newest first, without duplicates, and drops folders that no longer exist.
- It lives in `recents.json` in your user config directory, and the `x` on an entry forgets it.

![The splash screen after serving a map from the CLI, listing it under Recent as a map](../images/recent-maps-splash.png)

The list scrolls when it is long.
A map cached [from GitHub](open-from-github.md) also has a `clean up` button, which deletes the cached copy and the entry.
The `x` only forgets an entry and never touches files.
