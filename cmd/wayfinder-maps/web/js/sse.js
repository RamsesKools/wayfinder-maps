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

// progressText renders "N/M batches"; a zero total means not known yet.
export function progressText(batch, batches) {
  return batches > 0 ? batch + "/" + batches + " batches" : "starting…";
}

// readEvents pumps a fetch Response body through parseSSE, calling onEvent per
// event; it resolves when the stream ends.
export function readEvents(res, onEvent) {
  var reader = res.body.getReader(), dec = new TextDecoder(), buf = "";
  var pump = function() {
    return reader.read().then(function(c) {
      if (c.done) return;
      var p = parseSSE(buf + dec.decode(c.value, {stream: true})); buf = p.rest;
      p.events.forEach(onEvent);
      return pump();
    });
  };
  return pump();
}
