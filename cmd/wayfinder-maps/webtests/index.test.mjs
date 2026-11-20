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

// The export has no server: whatever needs one carries data-server-only, and one CSS rule hides it in static mode.
test("server-only UI is marked data-server-only", () => {
  for (const id of ["backbtn", "pullbtn", "pullacct", "pullstat", "updinfo", "splash", "maplist"]) {
    assert.match(html, new RegExp(`<[^>]*\\bid="${id}"[^>]*\\bdata-server-only\\b|<[^>]*\\bdata-server-only\\b[^>]*\\bid="${id}"`), `#${id} is not marked data-server-only`);
  }
  for (const input of ["ghaccount", "updatecheck"]) {
    assert.match(html, new RegExp(`<div class="setrow" data-server-only><label class="setname" for="${input}"`), `the ${input} settings row is not marked data-server-only`);
  }
});

test("one CSS rule hides data-server-only in static mode", () => {
  const css = readFileSync(new URL("../web/style.css", import.meta.url), "utf8");
  assert.match(css, /body\.static \[data-server-only\]\s*\{[^}]*display:\s*none\s*!important/);
});
