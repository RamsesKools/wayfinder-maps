// Header compass: the viewer's own dial re-orients the logo (and favicon) like it re-orients a map.
import {mountCompass} from "./viewer/compass.js";
import {NONE, flow, isDir} from "./viewer/direction.js";

const KEY = "wayfinder-logo-direction", DEFAULT = "NE", SPIN_MS = 500, FADE_MS = 150;

// The logo as drawn: a constellation flowing south-west to north-east, five stars from small to large.
const STARS = [[14.52, 49.48, 3.2], [30.48, 45.68, 3.8], [25.92, 33.52, 4.4], [39.6, 36.56, 5.2], [49.48, 16.8, 6.4]];
const LINES = [[0, 1], [0, 2], [0, 3], [3, 4], [2, 4], [1, 3]];
// Radial: the start in the middle, stars fanning out, one pair joined to each other.
const ring = (d, a, r) => [32 + d * Math.cos(a), 32 + d * Math.sin(a), r];
const RADIAL = {stars: [[32, 32, 3.2], ring(16, -1.2, 3.8), ring(19, 0.1, 4.4), ring(16, 1.9, 5.2), ring(20, 3.5, 6.4)], lines: [[0, 1], [0, 2], [0, 3], [3, 4]]};
const NE_ANGLE = Math.atan2(-1, 1);
const angle = (dir) => { const f = flow(dir); return Math.atan2(f.y, f.x); };

function layout(dir) {
  if (dir === NONE) return RADIAL;
  const a = angle(dir) - NE_ANGLE, c = Math.cos(a), s = Math.sin(a);
  const stars = STARS.map(([x, y, r]) => [32 + (x - 32) * c - (y - 33) * s, 33 + (x - 32) * s + (y - 33) * c, r]);
  return {stars, lines: LINES};
}

let seq = 0;
function logoSvg(dir) {
  const {stars, lines} = layout(dir), id = "wl" + seq++, glow = (r) => r * 2.3;
  const lo = [Math.min(...stars.map(([x, , r]) => x - glow(r))), Math.min(...stars.map(([, y, r]) => y - glow(r)))];
  const hi = [Math.max(...stars.map(([x, , r]) => x + glow(r))), Math.max(...stars.map(([, y, r]) => y + glow(r)))];
  const k = Math.min(1, 64 / Math.max(hi[0] - lo[0], hi[1] - lo[1]));
  const shift = (l, h) => Math.max(-k * l, 0) + Math.min(64 - (k * h + Math.max(-k * l, 0)), 0);
  const f = (v) => v.toFixed(2);
  const d = lines.map(([a, b]) => `M${f(stars[a][0])} ${f(stars[a][1])}L${f(stars[b][0])} ${f(stars[b][1])}`).join("");
  const dots = (fn) => stars.map(fn).join("");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs><radialGradient id="h${id}"><stop offset="0" stop-color="#38bdf8" stop-opacity=".95"/><stop offset=".35" stop-color="#0284c7" stop-opacity=".55"/><stop offset="1" stop-color="#0284c7" stop-opacity="0"/></radialGradient><filter id="b${id}" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="1.6"/></filter></defs>`
    + `<g transform="translate(${f(shift(lo[0], hi[0]))} ${f(shift(lo[1], hi[1]))}) scale(${k.toFixed(3)})">`
    + `<path d="${d}" fill="none" stroke="#0284c7" stroke-width="3.5" stroke-linecap="round" opacity=".55" filter="url(#b${id})"/>`
    + dots(([x, y, r]) => `<circle cx="${f(x)}" cy="${f(y)}" r="${f(glow(r))}" fill="url(#h${id})"/>`)
    + `<path d="${d}" fill="none" stroke="#38bdf8" stroke-width="1.6" stroke-linecap="round"/>`
    + dots(([x, y, r]) => `<circle cx="${f(x)}" cy="${f(y)}" r="${r}" fill="#38bdf8"/><circle cx="${f(x)}" cy="${f(y)}" r="${r / 2}" fill="#fff"/>`)
    + "</g></svg>";
}

const dataUrl = (dir) => "data:image/svg+xml," + encodeURIComponent(logoSvg(dir));
const load = () => { try { const d = localStorage.getItem(KEY); return isDir(d) ? d : DEFAULT; } catch { return DEFAULT; } };
const save = (d) => { try { localStorage.setItem(KEY, d); } catch { /* storage blocked: the choice lasts this page */ } };

function init(root) {
  const logos = [...document.querySelectorAll(".md-logo img")];
  const header = logos.find((i) => i.closest(".md-header"));
  const icons = [...document.querySelectorAll("link[rel~=icon]")];
  const still = matchMedia("(prefers-reduced-motion: reduce)");
  let shown = load(), timer = 0;

  const paintIcon = (d) => icons.forEach((l) => { l.type = "image/svg+xml"; l.href = dataUrl(d); });
  const paint = (d) => { const u = dataUrl(d); logos.forEach((i) => { i.src = u; }); shown = d; };
  const reset = (d) => {
    clearTimeout(timer);
    if (header) Object.assign(header.style, {transition: "none", transform: "", opacity: ""});
    paint(d);
  };

  root.setAttribute("role", "group");
  root.setAttribute("aria-label", "Logo direction");
  const set = mountCompass(root, pick);
  set(shown);
  if (shown !== DEFAULT) { paintIcon(shown); paint(shown); }

  function pick(d) {
    set(d); save(d); paintIcon(d);
    if (d === shown) return;
    clearTimeout(timer);
    if (still.matches || !header) return reset(d);
    if (d === NONE || shown === NONE) {
      Object.assign(header.style, {transition: `opacity ${FADE_MS}ms`, opacity: "0"});
      timer = setTimeout(() => {
        reset(d); header.style.opacity = "0";
        requestAnimationFrame(() => Object.assign(header.style, {transition: `opacity ${FADE_MS}ms`, opacity: "1"}));
      }, FADE_MS);
      return;
    }
    const turn = (((angle(d) - angle(shown)) * 180 / Math.PI + 540) % 360) - 180;
    Object.assign(header.style, {transition: `transform ${SPIN_MS}ms cubic-bezier(.3,1.3,.5,1)`, transform: `rotate(${turn}deg)`});
    timer = setTimeout(() => reset(d), SPIN_MS + 20);
  }
}

const dial = document.querySelector("[data-md-component=logo-compass]");
if (dial) init(dial);
