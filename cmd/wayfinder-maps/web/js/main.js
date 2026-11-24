// Boot: size the canvas, start the render loop and the poller, wire the
// top-level buttons, and open whatever /api/initial points at.
import {S, canvas, dpr} from "./state.js";
import {initStars, render} from "./draw.js";
import "./input.js"; // wires pan/zoom/click on the canvas
import "./panel.js"; // wires the panel close button and cross-ticket links
import {startPolling, showSplash, openProject, loadMap, loadStaticMap, leaveMap, pullFromGithub, setDirection} from "./screens.js";
import {mountGithub} from "./ghpicker.js";
import {mountCompass} from "./compass.js";
import {mountSettings} from "./settings.js";
import {DEFAULT, isDir} from "./direction.js";
import {isStatic, loadPrefs, localStore, zoomMode} from "./static.js";

// An exported site has no server: the map comes from graph.json and the server-only UI is hidden by CSS.
S.static = isStatic(document);
document.body.classList.toggle("static", S.static);

function resize() {
  canvas.width = innerWidth * dpr; canvas.height = innerHeight * dpr;
  canvas.style.width = innerWidth + "px"; canvas.style.height = innerHeight + "px";
}
window.addEventListener("resize", resize);

document.getElementById("openfolder").onclick = function() {
  fetch("/api/pick").then(function(r) { return r.json(); }).then(function(d) { if (d.path) openProject(d.path); });
};
document.getElementById("openanother").onclick = function() { leaveMap(showSplash); };
document.getElementById("backbtn").onclick = function() {
  leaveMap(function() { if (S.lastProject) openProject(S.lastProject); else showSplash(); });
};

document.getElementById("pullbtn").onclick = pullFromGithub;
mountGithub(function(path, refresh) { S.lastProject = null; loadMap(path); if (refresh) pullFromGithub(); });

// The compass rides above the HUD bar, whose height changes with its content and width.
var hudEl = document.getElementById("hud");
new ResizeObserver(function() { document.documentElement.style.setProperty("--hudh", hudEl.offsetHeight + "px"); }).observe(hudEl);

var setCompass = mountCompass(document.getElementById("compass"), function(d) { setDirection(d); setCompass(S.dir); });
setCompass(S.dir);
var syncSettings = mountSettings();

function bootStatic() {
  var prefs = loadPrefs(localStore());
  S.dir = prefs.direction; S.freeze = prefs.freeze; setCompass(S.dir); syncSettings();
  S.zoomButtons = zoomMode(location.search);
  document.body.classList.toggle("zoombuttons", S.zoomButtons);
  if (S.zoomButtons) document.getElementById("hint").textContent = "drag to pan \u00b7 press +/- to zoom \u00b7 click a star";
  loadStaticMap().catch(function(e) { console.error("wayfinder-maps: could not load graph.json", e); });
}

function bootServer() {
  startPolling();
  // The saved settings have to be known before the first layout runs.
  fetch("/api/settings").then(function(r) { return r.json(); }).then(function(st) {
    S.dir = isDir(st.direction) ? st.direction : DEFAULT; setCompass(S.dir);
    S.freeze = st.freezeNodes === true; S.updateCheck = st.updateCheck !== false; S.ghAccount = typeof st.ghAccount === "string" ? st.ghAccount : ""; syncSettings();
  }).catch(function() {}).then(function() { return fetch("/api/initial"); }).then(function(r) { return r.json(); }).then(function(init) {
    if (init.effort) loadMap(init.effort);
    else if (init.project) openProject(init.project);
    else showSplash();
  }).catch(showSplash);
}

resize(); initStars(); render();
if (S.static) bootStatic(); else bootServer();
