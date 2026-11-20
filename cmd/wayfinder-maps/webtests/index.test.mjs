import test from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";

const html = readFileSync(new URL("../web/index.html", import.meta.url), "utf8").replace(/<!--[\s\S]*?-->/g, "");

test("index.html tags are balanced", () => {
  for (const tag of ["div", "span", "label", "button", "h2"]) {
    const open = (html.match(new RegExp(`<${tag}[\\s>]`, "g")) || []).length;
    const close = (html.match(new RegExp(`</${tag}>`, "g")) || []).length;
    assert.equal(open, close, `<${tag}> opened ${open}x, closed ${close}x`);
  }
});

test("the screens and overlays are siblings of the top bar, not inside it", () => {
  const top = html.indexOf('<div id="topbar">');
  let depth = 0, end = -1;
  for (const m of html.slice(top).matchAll(/<div[\s>]|<\/div>/g)) {
    depth += m[0] === "</div>" ? -1 : 1;
    if (depth === 0) { end = top + m.index + m[0].length; break; }
  }
  assert.notEqual(end, -1, "#topbar never closes");
  for (const id of ["compass", "hud", "panel", "splash", "maplist"]) {
    assert.ok(html.indexOf(`id="${id}"`) > end, `#${id} sits inside #topbar`);
  }
});
