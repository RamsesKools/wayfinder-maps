import test from "node:test";
import assert from "node:assert/strict";
import {isStatic, loadPrefs, savePrefs, zoomMode} from "../web/js/static.js";

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
  assert.deepEqual(loadPrefs(memory()), {direction: "E", freeze: false});
});

test("savePrefs round-trips and merges partial updates", () => {
  const s = memory();
  savePrefs(s, {direction: "SW"});
  savePrefs(s, {freeze: true});
  assert.deepEqual(loadPrefs(s), {direction: "SW", freeze: true});
  savePrefs(s, {direction: "none"});
  assert.deepEqual(loadPrefs(s), {direction: "none", freeze: true});
});

test("loadPrefs ignores an invalid stored direction", () => {
  const s = memory();
  s.setItem("wayfinder.direction", "up");
  assert.equal(loadPrefs(s).direction, "E");
});

test("blocked storage is harmless", () => {
  assert.deepEqual(loadPrefs(blocked), {direction: "E", freeze: false});
  assert.doesNotThrow(() => savePrefs(blocked, {direction: "N"}));
  assert.deepEqual(loadPrefs(null), {direction: "E", freeze: false});
});

test("zoomMode is on only for zoom=buttons", () => {
  assert.equal(zoomMode("?zoom=buttons"), true);
  assert.equal(zoomMode("?a=1&zoom=buttons"), true);
  assert.equal(zoomMode("?zoom=wheel"), false);
  assert.equal(zoomMode(""), false);
});
