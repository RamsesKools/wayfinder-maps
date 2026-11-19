// Where each ticket sits along the flow. Dependency depth sets the map's length;
// creation order only moves a ticket forward within its slack, so a late ticket
// drifts toward the goal without ever stretching the map.
export var WAVE_GAP = 3600, WAVE_STEP = 0.5, MIN_SPAN = 2;

// waves numbers creation sessions: a new one starts after WAVE_GAP seconds of
// silence. Only the order counts, so a long break is one step like a short one.
// A ticket with no known creation time is in wave 0.
export function waves(nodes) {
  var timed = nodes.filter(function(n) { return n.created > 0; });
  timed.sort(function(a, b) { return a.created - b.created || a.num - b.num; });
  var w = {}, k = 0;
  timed.forEach(function(n, i) {
    if (i && n.created - timed[i - 1].created >= WAVE_GAP) k++;
    w[n.num] = k;
  });
  return w;
}

// columns places nodes using only the blocker edges among them:
//   earliest = dependency depth, latest = span - longest chain ahead,
//   column   = min(latest, max(wave * step, column of each blocker + 1)).
// A blocker's latest is always at least one less than its dependent's, so
// capping at latest never puts a ticket behind its blocker.
export function columns(nodes, wave) {
  var by = {}, ahead = {};
  nodes.forEach(function(n) { by[n.num] = n; ahead[n.num] = []; });
  function blockers(n) { return (n.blockers || []).filter(function(b) { return by[b] && b !== n.num; }); }
  nodes.forEach(function(n) { blockers(n).forEach(function(b) { ahead[b].push(n.num); }); });

  // Longest path over a DAG, memoised; a back-edge from a cycle on disk adds nothing.
  function longest(next) {
    var memo = {}, seen = {};
    function f(num) {
      if (memo[num] != null) return memo[num];
      if (seen[num]) return 0;
      seen[num] = true;
      var d = 0;
      next(num).forEach(function(m) { d = Math.max(d, f(m) + 1); });
      seen[num] = false;
      return (memo[num] = d);
    }
    return f;
  }
  var depth = longest(function(num) { return blockers(by[num]); });
  var height = longest(function(num) { return ahead[num]; });

  var maxWave = 0, maxDepth = 0;
  nodes.forEach(function(n) { maxWave = Math.max(maxWave, wave[n.num] || 0); maxDepth = Math.max(maxDepth, depth(n.num)); });
  var span = Math.max(maxDepth, MIN_SPAN);
  var step = maxWave ? Math.min(WAVE_STEP, span / maxWave) : 0;

  var col = {}, seen = {};
  function place(num) {
    if (col[num] != null) return col[num];
    if (seen[num]) return 0;
    seen[num] = true;
    var c = (wave[num] || 0) * step;
    blockers(by[num]).forEach(function(b) { c = Math.max(c, place(b) + 1); });
    seen[num] = false;
    return (col[num] = Math.max(depth(num), Math.min(span - height(num), c)));
  }
  nodes.forEach(function(n) { place(n.num); });
  return col;
}
