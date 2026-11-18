// A popup list under a button or input: the viewer's own dropdown, so it looks
// the same on every machine. Opening one closes the others; a click outside or
// Escape closes it.
var menus = [];

document.addEventListener("click", function(ev) {
  menus = menus.filter(function(m) { return m.box.isConnected; });
  menus.forEach(function(m) { if (!m.box.contains(ev.target)) m.close(); });
});

export function menu(box, btn, list) {
  var m = {
    box: box,
    isOpen: function() { return !list.hidden; },
    open: function() {
      menus.forEach(function(o) { if (o !== m) o.close(); });
      list.hidden = false; btn.setAttribute("aria-expanded", "true");
    },
    close: function() { list.hidden = true; btn.setAttribute("aria-expanded", "false"); },
    toggle: function() { if (m.isOpen()) m.close(); else m.open(); },
  };
  box.addEventListener("keydown", function(ev) { if (ev.key === "Escape") m.close(); });
  menus.push(m);
  return m;
}
