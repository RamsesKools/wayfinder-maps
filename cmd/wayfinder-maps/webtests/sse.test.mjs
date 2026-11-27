import test from "node:test";
import assert from "node:assert/strict";
import {parseSSE, progressText} from "../web/js/sse.js";

test("parseSSE returns whole events and keeps the partial tail", () => {
  const r = parseSSE('event: progress\ndata: {"done":4,"total":15}\n\nevent: done\ndata: {}\n\nevent: prog');
  assert.deepEqual(r.events, [
    {event: "progress", data: {done: 4, total: 15}},
    {event: "done", data: {}},
  ]);
  assert.equal(r.rest, "event: prog");
});

test("parseSSE tolerates bad data", () => {
  assert.deepEqual(parseSSE("event: error\ndata: nope\n\n").events, [{event: "error", data: {}}]);
});

test("progressText counts batches and follows a moving total", () => {
  assert.equal(progressText(0, 1), "0/1 batches");
  assert.equal(progressText(2, 3), "2/3 batches");
  assert.equal(progressText(0, 0), "starting…");
});
