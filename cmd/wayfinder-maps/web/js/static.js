// Static mode: an exported site has no server, so the map comes from graph.json
// and the few viewer preferences that still apply live in localStorage. No DOM, no state.
import {DEFAULT, isDir} from "./direction.js";

var DIR_KEY = "wayfinder.direction", FREEZE_KEY = "wayfinder.freeze";

export function localStore() {
  try { return localStorage; } catch (e) { return null; }
}

export function isStatic(doc) { return !!doc.querySelector('meta[name="wayfinder-static"]'); }

// Storage can throw (sandboxed iframes, blocked cookies); preferences then just do not persist.
export function loadPrefs(storage) {
  var dir = DEFAULT, freeze = false;
  try {
    var d = storage.getItem(DIR_KEY);
    if (isDir(d)) dir = d;
    freeze = storage.getItem(FREEZE_KEY) === "true";
  } catch (e) {}
  return {direction: dir, freeze: freeze};
}

export function savePrefs(storage, patch) {
  try {
    if (patch.direction !== undefined) storage.setItem(DIR_KEY, patch.direction);
    if (patch.freeze !== undefined) storage.setItem(FREEZE_KEY, patch.freeze);
  } catch (e) {}
}
