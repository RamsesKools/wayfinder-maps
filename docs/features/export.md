# Export to a static site

`wayfinder-maps export` takes a snapshot of a map and turns it into a small website.
It is the same interactive star-map: click a star, read its ticket, pan, zoom, turn the compass.
But it needs no binary and no server of yours, so you can put it anywhere: on a website, in a docs page, behind a link you send someone, or inside a presentation that can show a web page.

![A map exported to a static site, embedded in a web page](../images/demo-page.png)

The [demo](../demo.md) on this site is exactly that: an export of the test fixture map, embedded in this page.

## Export a map

```sh
wayfinder-maps export <map-dir> <out-dir>
```

`<map-dir>` is a map (a folder with `map.md` and `tickets/`), like the one `status` and `serve` take.
For a map kept as GitHub issues, [`pull`](pull.md) it first and export the folder it wrote.

```
site/
  index.html    the viewer page, flagged as static
  style.css
  js/           the viewer modules
  favicon.*
  graph.json    the snapshot of your map
```

## How it works

1. The export reads the map like `serve` does and builds the same graph the viewer normally fetches from the server, ticket text included.
   It writes that to `graph.json`.
2. It copies the viewer files baked into the binary and adds a `<meta name="wayfinder-static">` flag to `index.html`.
3. When the page opens and sees the flag, the viewer loads `graph.json` instead of asking a server, and never polls for changes.
   It hides what needs a server: Open Folder, recent maps, Open from GitHub, Refresh from GitHub and the back to Maps button.
   The [compass](compass.md) direction and Freeze nodes are kept in the browser's `localStorage`, and the gh account and update check rows of the [settings menu](settings.md) are hidden.

The `serve` and `app` viewer is unchanged.

## Put it somewhere

### Host it

Copy the output folder to any static host: GitHub Pages, Netlify, S3, a plain web server.
Link to the folder with a trailing slash (`https://example.com/my-map/`), since the page finds its files by relative path.

Open it over http(s), not by double-clicking `index.html`.
The page loads `graph.json` and ES modules, and browsers block both on `file://`.
For a quick local look, or to share on your network:

```sh
python3 -m http.server -d site
```

### Embed it in a page

Put the output folder next to your page, or host it separately, and point an iframe at it:

```html
<iframe src="my-map/" style="width:100%;height:600px;border:0" title="My wayfinder map"></iframe>
```

- It works from the same site or a different one.
- A `sandbox` attribute needs `allow-scripts allow-same-origin`, or the viewer cannot load its modules.
- A host that forbids framing (an `X-Frame-Options` or `frame-ancestors` header) blocks it.

### Presentations and other places

Anything that can show a web page can show the map, by iframe or by link.
That is as far as this goes for now: using it in specific slide tools, or in formats that cannot run a web page such as PDF or PowerPoint, is not worked out yet.

## Good to know

- The export is a snapshot.
  Re-run it to pick up edits to the map, for example in CI to keep a published copy fresh. This site does that for the demo on every `mkdocs build` (`docs_hooks.py`, which needs Go on the PATH).
- It includes every ticket's text, so only export maps you are happy to publish.
- `<out-dir>` is created if needed and files in it are overwritten.
  It cannot sit inside the map folder.
