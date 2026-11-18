// parseSSE splits buffered text into complete SSE events ({event, data}) and
// the unfinished remainder to prepend to the next chunk.
export function parseSSE(buf) {
  var parts = buf.split("\n\n"), rest = parts.pop(), events = [];
  parts.forEach(function(p) {
    var ev = {event: "message", data: ""};
    p.split("\n").forEach(function(l) {
      if (l.indexOf("event: ") === 0) ev.event = l.slice(7);
      else if (l.indexOf("data: ") === 0) ev.data += l.slice(6);
    });
    try { ev.data = JSON.parse(ev.data); } catch (e) { ev.data = {}; }
    events.push(ev);
  });
  return {events: events, rest: rest};
}

// progressText renders "N/M items"; a zero total means not known yet.
export function progressText(done, total) {
  return total > 0 ? done + "/" + total + " items" : "starting…";
}
