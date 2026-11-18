// The splash's "Open from GitHub" section: gh account, repo combo box, map
// dropdown and the pull that mirrors the chosen map into the user cache dir.
import {el} from "./util.js";
import {gateFor, mapLabel} from "./github.js";
import {readEvents, progressText} from "./sse.js";

var H = {headers: {"X-Wayfinder": "1"}};
var st = {gate: gateFor(null), account: "", maps: [], pick: null, scan: 0, busy: false, repo: "", typed: null};
var openMap = function() {};
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
    var sel = el("select", null); sel.id = "ghacct"; sel.title = "Switch gh account";
    s.accounts.forEach(function(a) { var o = el("option", null, a); o.value = a; sel.appendChild(o); });
    sel.value = st.account;
    sel.onchange = function() { st.account = sel.value; clearMaps(); loadRepos(); if ($("ghrepo").value.trim()) scan(true); };
    box.appendChild(sel);
  } else box.appendChild(el("b", null, "@" + s.account));
}

function clearMaps() { st.maps = []; st.pick = null; st.scan++; closeList(); renderList(); setStat("", ""); sync(); }

function loadRepos() {
  var dl = $("ghrepos"); dl.innerHTML = "";
  if (!st.gate.ok) return;
  getJSON("/api/github/repos?account=" + encodeURIComponent(st.account)).then(function(repos) {
    repos.forEach(function(r) { var o = document.createElement("option"); o.value = r; dl.appendChild(o); });
  }).catch(function() {});
}

// scan lists the maps of the typed repo; a counter drops answers from a repo
// the user has already moved on from.
function scan(force) {
  var typed = $("ghrepo").value.trim();
  if (force !== true && typed === st.typed) return; // blur and Enter re-fire for text already scanned
  st.typed = typed;
  var n = ++st.scan;
  st.maps = []; st.pick = null; closeList(); renderList(); sync();
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
    row.onclick = function() { st.pick = m; closeList(); sync(); };
    l.appendChild(row);
  });
}

function closeList() { $("ghmaplist").hidden = true; $("ghmapbtn").setAttribute("aria-expanded", "false"); }

function open() {
  var m = st.pick; if (!m || st.busy) return;
  st.busy = true; sync(); setStat("run", progressText(0, 0));
  var failed = false, path = "";
  var url = "/api/github/open?repo=" + encodeURIComponent(st.repo) + "&map=" + m.number + "&title=" + encodeURIComponent(m.title) + "&account=" + encodeURIComponent(st.account);
  fetch(url, {method: "POST", headers: H.headers}).then(function(r) {
    if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
    return readEvents(r, function(ev) {
      if (ev.event === "progress") setStat("run", progressText(ev.data.done, ev.data.total));
      else if (ev.event === "error") { failed = true; setStat("err", (ev.data.message || "Pull failed")); }
      else if (ev.event === "done") path = ev.data.path;
    });
  }).then(function() {
    if (failed) return;
    if (!path) throw new Error("Pull ended without finishing");
    setStat("", ""); openMap(path);
  }).catch(function(e) { setStat("err", e.message); }).then(function() { st.busy = false; sync(); });
}

// refreshGithub re-reads the gh status each time the splash shows, so logging
// in or out in a terminal is picked up without a restart.
export function refreshGithub() {
  setStat("", "");
  getJSON("/api/github/status").catch(function() { return null; }).then(function(s) {
    renderStatus(s);
    if (s && s.loggedIn && !s.accounts.includes(st.account)) st.account = s.account;
    if (s && s.loggedIn) { var sel = $("ghacct"); if (sel) sel.value = st.account; }
    sync(); loadRepos();
    if (st.gate.ok && $("ghrepo").value.trim() && !st.maps.length) scan(true);
  });
}

export function mountGithub(onOpen) {
  openMap = onOpen;
  var repo = $("ghrepo"), timer = null;
  // A datalist pick or a blur fires "change"; typing or pasting a full repo
  // scans after a short pause, so no explicit submit is needed.
  repo.onchange = function() { scan(); };
  repo.oninput = function() { clearTimeout(timer); timer = setTimeout(function() { if (/^\S+\/\S+$|github\.com/.test(repo.value.trim())) scan(); }, 500); };
  repo.onkeydown = function(ev) { if (ev.key === "Enter") { clearTimeout(timer); scan(true); } };
  $("ghmapbtn").onclick = function() { var l = $("ghmaplist"); l.hidden = !l.hidden; this.setAttribute("aria-expanded", String(!l.hidden)); };
  document.addEventListener("click", function(ev) { if (!$("ghmap").contains(ev.target)) closeList(); });
  $("ghopen").onclick = open;
  sync();
}
