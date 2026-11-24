import test from "node:test";
import assert from "node:assert/strict";
import {isStatic, loadPrefs, savePrefs, zoomOverride} from "../web/js/static.js";

const memory = () => {
  const m = new Map();
  return {getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => m.set(k, String(v))};
};
const blocked = {
  getItem() { throw new Error("SecurityError"); },
  setItem() { throw new Error("SecurityError"); },
};

test("isStatic follows the wayfinder-static meta", () => {
  const withMeta = {querySelector: (sel) => (sel === 'meta[name="wayfinder-static"]' ? {} : null)};
  const without = {querySelector: () => null};
  assert.equal(isStatic(withMeta), true);
  assert.equal(isStatic(without), false);
});

test("loadPrefs falls back to the defaults", () => {
  assert.deepEqual(loadPrefs(memory()), {direction: "E", freeze: false, zoomButtons: false});
});

test("savePrefs round-trips and merges partial updates", () => {
  const s = memory();
  savePrefs(s, {direction: "SW"});
  savePrefs(s, {freezeNodes: true});
  assert.deepEqual(loadPrefs(s), {direction: "SW", freeze: true, zoomButtons: false});
  savePrefs(s, {direction: "none"});
  assert.deepEqual(loadPrefs(s), {direction: "none", freeze: true, zoomButtons: false});
});

test("loadPrefs ignores an invalid stored direction", () => {
  const s = memory();
  s.setItem("wayfinder.direction", "up");
  assert.equal(loadPrefs(s).direction, "E");
});

test("blocked storage is harmless", () => {
  assert.deepEqual(loadPrefs(blocked), {direction: "E", freeze: false, zoomButtons: false});
  assert.doesNotThrow(() => savePrefs(blocked, {direction: "N"}));
  assert.deepEqual(loadPrefs(null), {direction: "E", freeze: false, zoomButtons: false});
});

test("zoomButtons is saved and loaded", () => {
  const s = memory();
  assert.equal(loadPrefs(s).zoomButtons, false);
  savePrefs(s, {zoomButtons: true});
  assert.equal(loadPrefs(s).zoomButtons, true);
  savePrefs(s, {direction: "N"});
  assert.equal(loadPrefs(s).zoomButtons, true);
  savePrefs(s, {zoomButtons: false});
  assert.equal(loadPrefs(s).zoomButtons, false);
});

test("zoomOverride reads zoom=buttons and zoom=wheel, anything else is no override", () => {
  assert.equal(zoomOverride("?zoom=buttons"), true);
  assert.equal(zoomOverride("?a=1&zoom=buttons"), true);
  assert.equal(zoomOverride("?zoom=wheel"), false);
  assert.equal(zoomOverride("?zoom=other"), null);
  assert.equal(zoomOverride(""), null);
});
