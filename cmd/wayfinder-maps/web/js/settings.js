// The gear popover in the top bar: home for viewer toggles.
import {S} from "./state.js";
import {menu} from "./menu.js";

function save(patch) {
  fetch("/api/settings", {method: "POST", headers: {"X-Wayfinder": "1"}, body: JSON.stringify(patch)}).catch(function() {});
}

export function mountSettings() {
  var m = menu(document.getElementById("settings"), document.getElementById("setbtn"), document.getElementById("setpop"));
  document.getElementById("setbtn").onclick = m.toggle;
  var freeze = document.getElementById("freezenodes");
  freeze.onchange = function() { S.freeze = freeze.checked; save({freezeNodes: S.freeze}); };
  return function() { freeze.checked = S.freeze; };
}
