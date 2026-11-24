import test from "node:test";
import assert from "node:assert/strict";
import {isStatic, loadPrefs, savePrefs, overrides} from "../web/js/static.js";

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

test("overrides reads zoom, freeze and compass from the address, anything else is no override", () => {
  const none = {zoom: null, freeze: null, dir: null};
  assert.deepEqual(overrides(""), none);
  assert.deepEqual(overrides("?zoom=other&freeze=maybe&compass=up"), none);
  assert.equal(overrides("?zoom=buttons").zoom, true);
  assert.equal(overrides("?a=1&zoom=buttons").zoom, true);
  assert.equal(overrides("?zoom=wheel").zoom, false);
  assert.equal(overrides("?freeze=on").freeze, true);
  assert.equal(overrides("?freeze=off").freeze, false);
  assert.equal(overrides("?compass=SW").dir, "SW");
  assert.equal(overrides("?compass=sw").dir, "SW");
  assert.equal(overrides("?compass=none").dir, "none");
  assert.equal(overrides("?compass=NONE").dir, "none");
  assert.deepEqual(overrides("?zoom=buttons&freeze=on&compass=n"), {zoom: true, freeze: true, dir: "N"});
});
