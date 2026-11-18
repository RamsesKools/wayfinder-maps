import test from "node:test";
import assert from "node:assert/strict";
import {gateFor, mapLabel, filterRepos, stepIndex, GH_INSTALL_URL} from "../web/js/github.js";
import {readEvents} from "../web/js/sse.js";

test("gateFor enables the button only for an installed, logged-in gh", () => {
  assert.deepEqual(gateFor({installed: true, loggedIn: true, account: "alice"}), {ok: true, hint: ""});
});

test("gateFor hints at the install page when gh is missing", () => {
  const g = gateFor({installed: false, loggedIn: false});
  assert.equal(g.ok, false);
  assert.equal(g.link, GH_INSTALL_URL);
  assert.equal(GH_INSTALL_URL, "https://cli.github.com");
  assert.match(g.hint, /not installed/);
});

test("gateFor hints at gh auth login when not logged in", () => {
  const g = gateFor({installed: true, loggedIn: false});
  assert.equal(g.ok, false);
  assert.match(g.hint, /gh auth login/);
});

test("gateFor disables the button when the status could not be read", () => {
  assert.equal(gateFor(null).ok, false);
});

test("mapLabel shows name and percent completed, and flags closed maps", () => {
  assert.equal(mapLabel({title: "Live map", percent: 75, closed: false}), "Live map (75%)");
  assert.equal(mapLabel({title: "Old", percent: 100, closed: true}), "Old (100%, closed)");
});

test("readEvents feeds events split across chunks and resolves at the end", async () => {
  const enc = new TextEncoder();
  const body = new ReadableStream({start(c) {
    c.enqueue(enc.encode('event: progress\ndata: {"done":1,"total":2}\n\nevent: do'));
    c.enqueue(enc.encode('ne\ndata: {"path":"/x"}\n\n'));
    c.close();
  }});
  const got = [];
  await readEvents(new Response(body), ev => got.push(ev));
  assert.deepEqual(got, [
    {event: "progress", data: {done: 1, total: 2}},
    {event: "done", data: {path: "/x"}},
  ]);
});

test("filterRepos keeps repos containing the typed text, ignoring case, and all when nothing is typed", () => {
  const repos = ["acme/Widgets", "acme/gadgets", "alice/notes"];
  assert.deepEqual(filterRepos(repos, "WID"), ["acme/Widgets"]);
  assert.deepEqual(filterRepos(repos, "acme/"), ["acme/Widgets", "acme/gadgets"]);
  assert.deepEqual(filterRepos(repos, "  "), repos);
  assert.deepEqual(filterRepos(repos, "zzz"), []);
});

test("stepIndex walks the rows and wraps around", () => {
  assert.equal(stepIndex(-1, 3, 1), 0);
  assert.equal(stepIndex(-1, 3, -1), 2);
  assert.equal(stepIndex(2, 3, 1), 0);
  assert.equal(stepIndex(0, 3, -1), 2);
  assert.equal(stepIndex(-1, 0, 1), -1);
});
