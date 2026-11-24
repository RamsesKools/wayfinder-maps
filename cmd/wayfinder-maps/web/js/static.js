// Static mode: an exported site has no server, so the map comes from graph.json
// and the few viewer preferences that still apply live in localStorage. No DOM, no state.
import {DEFAULT, isDir} from "./direction.js";

var DIR_KEY = "wayfinder.direction", FREEZE_KEY = "wayfinder.freeze", ZOOM_KEY = "wayfinder.zoomButtons";

export function localStore() {
  try { return localStorage; } catch (e) { return null; }
}

// ?zoom=buttons or ?zoom=wheel overrides the saved setting for this page load; anything else is no override.
export function zoomOverride(search) {
  var z = new URLSearchParams(search).get("zoom");
  return z === "buttons" ? true : z === "wheel" ? false : null;
}

export function isStatic(doc) { return !!doc.querySelector('meta[name="wayfinder-static"]'); }

// Storage can throw (sandboxed iframes, blocked cookies); preferences then just do not persist.
export function loadPrefs(storage) {
  var dir = DEFAULT, freeze = false, zoomButtons = false;
  try {
    var d = storage.getItem(DIR_KEY);
    if (isDir(d)) dir = d;
    freeze = storage.getItem(FREEZE_KEY) === "true";
    zoomButtons = storage.getItem(ZOOM_KEY) === "true";
  } catch (e) {}
  return {direction: dir, freeze: freeze, zoomButtons: zoomButtons};
}

export function savePrefs(storage, patch) {
  try {
    if (patch.direction !== undefined) storage.setItem(DIR_KEY, patch.direction);
    if (patch.freezeNodes !== undefined) storage.setItem(FREEZE_KEY, patch.freezeNodes);
    if (patch.zoomButtons !== undefined) storage.setItem(ZOOM_KEY, patch.zoomButtons);
  } catch (e) {}
}
