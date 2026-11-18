// Pure helpers for the splash's "Open from GitHub" section.
export const GH_INSTALL_URL = "https://cli.github.com";

// gateFor says whether Open from GitHub is usable, given /api/github/status
// (null when it could not be read), and otherwise what to tell the user.
export function gateFor(st) {
  if (!st) return {ok: false, hint: "Could not check the gh CLI."};
  if (!st.installed) return {ok: false, hint: "The gh CLI is not installed. Install it from", link: GH_INSTALL_URL};
  if (!st.loggedIn) return {ok: false, hint: "gh is not logged in. Run `gh auth login` in a terminal."};
  return {ok: true, hint: ""};
}

// mapLabel is a map's dropdown text: name and percent completed.
export function mapLabel(m) {
  return m.title + " (" + m.percent + "%" + (m.closed ? ", closed" : "") + ")";
}

// filterRepos narrows the repo suggestions to those containing what was typed.
export function filterRepos(repos, typed) {
  var q = typed.trim().toLowerCase();
  return q ? repos.filter(function(r) { return r.toLowerCase().includes(q); }) : repos;
}

// stepIndex moves the highlighted row by dir (+1 or -1), wrapping; -1 means none yet.
export function stepIndex(i, n, dir) {
  if (!n) return -1;
  if (i < 0) return dir > 0 ? 0 : n - 1;
  return (i + dir + n) % n;
}
