import { test } from "node:test";
import assert from "node:assert/strict";
import { waves, columns } from "../web/js/columns.js";

const H = 3600, DAY = 24 * H;
const node = (num, blockers, created) => ({ num, blockers, created });

test("waves split on an hour of silence and ignore how long the silence was", () => {
  const t0 = 1_700_000_000;
  const ns = [
    node(1, [], t0), node(2, [], t0 + 0.4 * H), node(3, [], t0 + 0.9 * H),
    node(4, [], t0 + 2.2 * H), node(5, [], t0 + 2.5 * H),
    node(6, [], t0 + 2 * DAY), node(7, [], 0),
  ];
  assert.deepEqual(waves(ns), { 1: 0, 2: 0, 3: 0, 4: 1, 5: 1, 6: 2 });
});

// The example from the design: 01,10 -> 03 -> 04 -> 06 is the longest chain,
// 07 -> 09 has slack, 10 is a blocker found two days in.
function example() {
  const t0 = 1_700_000_000, w1 = t0 + 2 * H, w2 = t0 + 2 * DAY;
  return [
    node(1, [], t0), node(2, [], t0), node(3, [1, 10], t0), node(4, [3], t0), node(5, [], t0),
    node(6, [4], w1), node(7, [], w1),
    node(8, [], w2), node(9, [7], w2), node(10, [], w2),
  ];
}

test("age moves tickets only within their slack, so the map keeps its length", () => {
  const ns = example();
  const c = columns(ns, waves(ns));
  assert.deepEqual(c, { 1: 0, 2: 0, 3: 1, 4: 2, 5: 0, 6: 3, 7: 0.5, 8: 1, 9: 1.5, 10: 0 });
});

test("without creation times the columns are the dependency depth", () => {
  const ns = example().map((n) => ({ ...n, created: 0 }));
  assert.deepEqual(columns(ns, waves(ns)), { 1: 0, 2: 0, 3: 1, 4: 2, 5: 0, 6: 3, 7: 0, 8: 0, 9: 1, 10: 0 });
});

test("a flat map still spreads by age, up to a minimum span of 2", () => {
  const ns = [0, 1, 2, 3, 4, 5].map((k) => node(k + 1, [], 1_700_000_000 + k * DAY));
  const c = columns(ns, waves(ns));
  assert.deepEqual(Object.values(c).map((v) => +v.toFixed(6)), [0, 0.4, 0.8, 1.2, 1.6, 2]);
});

test("many waves shrink the step so the map never stretches", () => {
  const ns = [];
  for (let k = 0; k < 40; k++) ns.push(node(k + 1, k % 4 ? [k] : [], 1_700_000_000 + k * DAY));
  const c = columns(ns, waves(ns));
  const span = Math.max(...Object.values(c));
  assert.equal(span, 3);
  ns.forEach((n) => n.blockers.forEach((b) => assert.ok(c[n.num] >= c[b] + 1, `${b} -> ${n.num}`)));
});

test("a blocker cycle on disk still places every ticket", () => {
  const ns = [node(1, [2], 1), node(2, [1], 2 * H), node(3, [], 5 * H)];
  const c = columns(ns, waves(ns));
  ns.forEach((n) => assert.ok(Number.isFinite(c[n.num]), `${n.num}`));
});
