// The splash's "Open from GitHub" section: gh account, repo combo box, map
// dropdown and the pull that mirrors the chosen map into the user cache dir.
import {el} from "./util.js";
import {gateFor, mapLabel, filterRepos, stepIndex} from "./github.js";
import {menu} from "./menu.js";
import {readEvents, progressText} from "./sse.js";

var H = {headers: {"X-Wayfinder": "1"}};
var st = {gate: gateFor(null), account: "", maps: [], repos: [], hi: -1, pick: null, scan: 0, busy: false, repo: "", typed: null};
var openMap = function() {};
var repoMenu, mapMenu;
var $ = function(id) { return document.getElementById(id); };

function getJSON(url) {
  return fetch(url, H).then(function(r) {
    if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim() || "Request failed"); });
    return r.json();
  });
}

function setStat(kind, text) {
  var s = $("ghstat"); s.hidden = !kind; s.className = kind; s.title = text || "";
  s.querySelector(".txt").textContent = text || "";
}

function sync() {
  var ok = st.gate.ok;
  $("gh").classList.toggle("off", !ok);
  $("ghrepo").disabled = !ok || st.busy;
  $("ghmapbtn").disabled = !ok || st.busy || !st.maps.length;
  $("ghopen").disabled = !ok || st.busy || !st.pick;
  var b = $("ghmapbtn");
  $("ghrepobtn").disabled = !ok || st.busy;
  b.textContent = st.pick ? mapLabel(st.pick) : (st.maps.length ? "Choose a map…" : "No map to choose yet");
  b.classList.toggle("closed", !!(st.pick && st.pick.closed));
}

function renderStatus(s) {
  var box = $("ghstatus"); box.innerHTML = "";
  st.gate = gateFor(s);
  if (!st.gate.ok) {
    box.className = "hint";
    box.appendChild(document.createTextNode(st.gate.hint + " "));
    if (st.gate.link) {
      var a = el("a", null, st.gate.link); a.href = st.gate.link;
      a.onclick = function(ev) { ev.preventDefault(); fetch("/api/open", {method: "POST", headers: H.headers, body: st.gate.link}).catch(function() {}); };
      box.appendChild(a);
      box.appendChild(document.createTextNode(", then run `gh auth login`."));
    }
    return;
  }
  box.className = "";
  box.appendChild(document.createTextNode("Signed in as "));
  if (s.accounts.length > 1) {
    var wrap = el("span", null); wrap.id = "ghacctbox";
    var btn = el("button", null, "@" + st.account); btn.id = "ghacct"; btn.type = "button"; btn.title = "Switch gh account";
    btn.setAttribute("aria-haspopup", "listbox"); btn.setAttribute("aria-expanded", "false");
    var list = el("div", "ghlist"); list.id = "ghacctlist"; list.setAttribute("role", "listbox"); list.hidden = true;
    var m = menu(wrap, btn, list);
    s.accounts.forEach(function(a) {
      var row = el("button", "ghopt" + (a === st.account ? " cur" : ""), "@" + a); row.type = "button"; row.setAttribute("role", "option");
      row.onclick = function() {
        m.close();
        if (a === st.account) return;
        st.account = a; btn.textContent = "@" + a;
        list.querySelectorAll(".ghopt").forEach(function(r) { r.classList.toggle("cur", r === row); });
        clearMaps(); loadRepos(); if ($("ghrepo").value.trim()) scan(true);
      };
      list.appendChild(row);
    });
    btn.onclick = m.toggle;
    wrap.appendChild(btn); wrap.appendChild(list); box.appendChild(wrap);
  } else box.appendChild(el("b", null, "@" + s.account));
}

function clearMaps() { st.maps = []; st.pick = null; st.scan++; mapMenu.close(); renderList(); setStat("", ""); sync(); }

function loadRepos() {
  st.repos = []; renderRepos("");
  if (!st.gate.ok) return;
  getJSON("/api/github/repos?account=" + encodeURIComponent(st.account)).then(function(repos) {
    st.repos = repos; if (repoMenu.isOpen()) renderRepos($("ghrepo").value);
  }).catch(function() {});
}

// renderRepos fills the repo list with the suggestions matching filter.
function renderRepos(filter) {
  var l = $("ghrepolist"); l.innerHTML = ""; st.hi = -1;
  var rows = filterRepos(st.repos, filter);
  rows.forEach(function(r) {
    var row = el("button", "ghopt", r); row.type = "button"; row.setAttribute("role", "option");
    row.onclick = function() { pickRepo(r); };
    l.appendChild(row);
  });
  if (!rows.length && !filter.trim()) { var none = el("span", "ghopt none", "No recent repos yet. Paste one above."); l.appendChild(none); }
  return rows.length;
}

function pickRepo(r) { var inp = $("ghrepo"); inp.value = r; repoMenu.close(); scan(); }

function highlight(dir) {
  var rows = $("ghrepolist").querySelectorAll("button.ghopt");
  st.hi = stepIndex(st.hi, rows.length, dir);
  rows.forEach(function(r, i) { r.classList.toggle("on", i === st.hi); });
  if (rows[st.hi]) rows[st.hi].scrollIntoView({block: "nearest"});
}

// scan lists the maps of the typed repo; a counter drops answers from a repo
// the user has already moved on from.
function scan(force) {
  var typed = $("ghrepo").value.trim();
  if (force !== true && typed === st.typed) return; // blur and Enter re-fire for text already scanned
  st.typed = typed;
  var n = ++st.scan;
  st.maps = []; st.pick = null; mapMenu.close(); renderList(); sync();
  if (!typed || !st.gate.ok) { setStat("", ""); return; }
  setStat("run", "Scanning for maps…");
  getJSON("/api/github/maps?repo=" + encodeURIComponent(typed) + "&account=" + encodeURIComponent(st.account)).then(function(res) {
    if (n !== st.scan) return;
    st.repo = res.repo; st.maps = res.maps;
    if (res.maps.length === 1) st.pick = res.maps[0];
    setStat(res.maps.length ? "" : "err", res.maps.length ? "" : "No wayfinder maps found in " + res.repo);
    renderList(); sync();
  }).catch(function(e) { if (n === st.scan) { setStat("err", e.message); sync(); } });
}

function renderList() {
  var l = $("ghmaplist"); l.innerHTML = "";
  st.maps.forEach(function(m) {
    var row = el("button", "ghm" + (m.closed ? " closed" : "")); row.type = "button"; row.setAttribute("role", "option");
    row.appendChild(el("span", "ghmn", m.title));
    row.appendChild(el("span", "ghmp", m.percent + "%" + (m.closed ? " closed" : "")));
    var bar = el("span", "ghmb"), fill = document.createElement("i"); fill.style.width = m.percent + "%"; bar.appendChild(fill); row.appendChild(bar);
    row.onclick = function() { st.pick = m; mapMenu.close(); sync(); };
    l.appendChild(row);
  });
}

// open shows a map already in the cache at once and has the viewer refresh it
// from GitHub; only a map never opened before is pulled first.
function open() {
  var m = st.pick; if (!m || st.busy) return;
  st.busy = true; sync(); setStat("run", "Opening…");
  var ids = "repo=" + encodeURIComponent(st.repo) + "&map=" + m.number;
  getJSON("/api/github/cached?" + ids).catch(function() { return {cached: false}; }).then(function(c) {
    if (c.cached) { setStat("", ""); openMap(c.path, true); return; }
    return pull(m, ids);
  }).catch(function(e) { setStat("err", e.message); }).then(function() { st.busy = false; sync(); });
}

function pull(m, ids) {
  setStat("run", progressText(0, 0));
  var failed = false, path = "";
  var url = "/api/github/open?" + ids + "&title=" + encodeURIComponent(m.title) + "&account=" + encodeURIComponent(st.account);
  return fetch(url, {method: "POST", headers: H.headers}).then(function(r) {
    if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
    return readEvents(r, function(ev) {
      if (ev.event === "progress") setStat("run", progressText(ev.data.done, ev.data.total));
      else if (ev.event === "error") { failed = true; setStat("err", (ev.data.message || "Pull failed")); }
      else if (ev.event === "done") path = ev.data.path;
    });
  }).then(function() {
    if (failed) return;
    if (!path) throw new Error("Pull ended without finishing");
    setStat("", ""); openMap(path, false);
  });
}

// refreshGithub re-reads the gh status each time the splash shows, so logging
// in or out in a terminal is picked up without a restart.
export function refreshGithub() {
  setStat("", "");
  getJSON("/api/github/status").catch(function() { return null; }).then(function(s) {
    if (s && s.loggedIn && !s.accounts.includes(st.account)) st.account = s.account;
    renderStatus(s);
    sync(); loadRepos();
    if (st.gate.ok && $("ghrepo").value.trim() && !st.maps.length) scan(true);
  });
}

export function mountGithub(onOpen) {
  openMap = onOpen;
  var repo = $("ghrepo"), timer = null;
  repoMenu = menu($("ghrepobox"), $("ghrepobtn"), $("ghrepolist"));
  mapMenu = menu($("ghmap"), $("ghmapbtn"), $("ghmaplist"));
  $("ghrepobtn").onclick = function() {
    if (repoMenu.isOpen()) return repoMenu.close();
    renderRepos(""); repoMenu.open(); repo.focus();
  };
  $("ghrepolist").onmousedown = function(ev) { ev.preventDefault(); }; // keep focus in the input
  repo.onclick = function() { if (!repo.value.trim() && !repoMenu.isOpen()) { renderRepos(""); repoMenu.open(); } };
  // Blur fires "change"; typing or pasting a full repo scans after a short
  // pause, so no explicit submit is needed.
  repo.onchange = function() { scan(); };
  repo.oninput = function() {
    clearTimeout(timer);
    var typed = repo.value.trim().toLowerCase(), n = renderRepos(repo.value);
    // A complete name needs no suggestion, and its list would cover the map box.
    if (n && !(n === 1 && st.repos.some(function(r) { return r.toLowerCase() === typed; }))) repoMenu.open(); else repoMenu.close();
    timer = setTimeout(function() { if (/^\S+\/\S+$|github\.com/.test(repo.value.trim())) scan(); }, 500);
  };
  repo.onkeydown = function(ev) {
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      if (!repoMenu.isOpen()) { renderRepos(repo.value); repoMenu.open(); }
      highlight(ev.key === "ArrowDown" ? 1 : -1);
    } else if (ev.key === "Enter") {
      clearTimeout(timer);
      var on = $("ghrepolist").querySelector(".ghopt.on");
      if (repoMenu.isOpen() && on) pickRepo(on.textContent); else { repoMenu.close(); scan(true); }
    }
  };
  $("ghmapbtn").onclick = mapMenu.toggle;
  $("ghopen").onclick = open;
  sync();
}
