// The gear popover in the top bar: home for viewer toggles.
import {S} from "./state.js";
import {menu} from "./menu.js";
import {savePrefs, localStore} from "./static.js";

function save(patch) {
  if (S.static) { savePrefs(localStore(), patch); return Promise.resolve({ok: true}); }
  return fetch("/api/settings", {method: "POST", headers: {"X-Wayfinder": "1"}, body: JSON.stringify(patch)}).catch(function() {});
}

export function mountSettings() {
  var m = menu(document.getElementById("settings"), document.getElementById("setbtn"), document.getElementById("setpop"));
  document.getElementById("setbtn").onclick = m.toggle;
  var freeze = document.getElementById("freezenodes");
  freeze.onchange = function() { S.freeze = freeze.checked; save({freezeNodes: S.freeze}); };
  var upd = document.getElementById("updatecheck");
  upd.onchange = function() { S.updateCheck = upd.checked; save({updateCheck: S.updateCheck}).then(showUpdate); };
  var acct = document.getElementById("ghaccount");
  acct.oninput = function() { acct.classList.remove("bad"); };
  acct.onchange = function() {
    var v = acct.value.trim();
    save({ghAccount: v}).then(function(r) {
      if (r && r.ok) { S.ghAccount = v; acct.value = v; } else { acct.classList.add("bad"); }
    });
  };
  if (S.static) ["ghaccount", "updatecheck"].forEach(function(id) { document.getElementById(id).closest(".setrow").hidden = true; });
  return function() { freeze.checked = S.freeze; upd.checked = S.updateCheck; acct.value = S.ghAccount; showUpdate(); };
}

// The version line comes from the server's cached lookup, so this never costs a GitHub request.
function showUpdate() {
  if (S.static) return;
  fetch("/api/update").then(function(r) { return r.json(); }).then(function(u) { renderUpdate(u); }).catch(function() {});
}

export function renderUpdate(u) {
  var box = document.getElementById("updinfo"), ver = document.getElementById("updver"), hint = document.getElementById("updhint");
  var cur = u.current === "dev" ? "dev" : "v" + u.current;
  var text = cur;
  if (u.enabled && u.latest) text = cur + " (latest " + u.latest + ")";
  ver.textContent = text;
  box.classList.toggle("old", !!u.outdated);
  hint.hidden = !(u.enabled && u.outdated && u.hint);
  hint.textContent = u.hint ? "Update: " + u.hint : "";
  box.hidden = false;
}
