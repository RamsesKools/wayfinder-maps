// Star sky (dark) and printed star chart (light): parallax layers that twinkle and drift very slightly.
// Colors, alphas and mark shapes come from the --sf-* tokens in custom.css.
(() => {
  const LAYERS = [[0.15, 220, 0.7], [0.30, 120, 1.1], [0.50, 50, 1.8]];
  const CONSTELLATIONS = 6, DOT = 2.4;
  const DRIFT = 2.5, TWINKLE = 1.4, SHOOT_SPEED = 500;
  // Random gaps: 10s minimum plus an exponential tail, 60s on average.
  const nextGap = () => 10000 + -Math.log(1 - Math.random()) * 50000;
  const TRAIL = 110, CORNER = 44, TRIPLE_MS = 900;
  const still = matchMedia("(prefers-reduced-motion: reduce)");


  const canvas = document.createElement("canvas");
  canvas.className = "starfield";
  canvas.setAttribute("aria-hidden", "true");
  document.documentElement.appendChild(canvas);
  const ctx = canvas.getContext("2d");

  function rng(seed) {
    return () => {
      seed = (seed + 0x6d2b79f5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }
  const rnd = rng(9001);
  const layers = LAYERS.map(([f, n, sz]) => ({
    f, sz,
    stars: Array.from({ length: n }, () => ({ x: rnd(), y: rnd(), t: rnd(), c: Math.floor(rnd() * 3) })),
  }));

  // A short chain of bright stars per cell of a 3x2 grid, positioned relative to an anchor.
  const sky = Array.from({ length: CONSTELLATIONS }, (_, i) => {
    let x = 0, y = 0;
    const nodes = [{ x, y, t: rnd() }];
    for (let k = 3 + Math.floor(rnd() * 3); k > 0; k--) {
      const a = rnd() * 6.2832, d = 50 + rnd() * 70;
      x += Math.cos(a) * d; y += Math.sin(a) * d;
      nodes.push({ x, y, t: rnd() });
    }
    const xs = nodes.map((n) => n.x), ys = nodes.map((n) => n.y);
    const minX = Math.min(...xs), minY = Math.min(...ys);
    nodes.forEach((n) => { n.x -= minX; n.y -= minY; });
    return {
      nodes, w: Math.max(...xs) - minX, h: Math.max(...ys) - minY,
      ax: ((i % 3) + rnd()) / 3, ay: (Math.floor(i / 3) + rnd()) / 2,
    };
  });

  const pal = { stars: [], alpha: [], shape: "square", bright: "square", lines: "", trail: null, head: "", halo: null };
  const SPEC = document.createElement("canvas").getContext("2d");
  // Resolves any CSS color to its rgb parts and alpha, so one token can be faded to transparent.
  function parse(c) {
    SPEC.fillStyle = "#000"; SPEC.fillStyle = c;
    const v = SPEC.fillStyle;
    if (v[0] === "#") return { rgb: [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16)), a: 1 };
    const [r, g, b, a = 1] = v.match(/[\d.]+/g).map(Number);
    return { rgb: [r, g, b], a };
  }
  const fade = (c, k = 1) => `rgba(${c.rgb.join(",")},${c.a * k})`;

  function readPalette() {
    const cs = getComputedStyle(document.body);
    const get = (n) => cs.getPropertyValue(n).trim();
    pal.stars = get("--sf-stars").split(/\s+/).filter(Boolean);
    pal.alpha = get("--sf-alpha").split(/\s+/).map(Number);
    pal.shape = get("--sf-shape");
    pal.bright = get("--sf-bright");
    pal.lines = get("--sf-lines");
    pal.trail = parse(get("--sf-trail"));
    pal.head = get("--sf-head");
    pal.halo = parse(get("--sf-halo"));
  }

  let W = 0, H = 0, raf = 0, shot = null, nextShot = 0;
  const mod = (v, m) => ((v % m) + m) % m;

  function resize() {
    const dpr = window.devicePixelRatio || 1;
    W = innerWidth; H = innerHeight;
    canvas.width = W * dpr; canvas.height = H * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw(performance.now());
  }

  // One star crossing the sky from a random edge point to another edge, with a short fading trail.
  function startShot(now) {
    const edge = (e, r) => [[r * W, 0], [W, r * H], [r * W, H], [0, r * H]][e];
    const e0 = Math.floor(Math.random() * 4), e1 = (e0 + 1 + Math.floor(Math.random() * 3)) % 4;
    const [x0, y0] = edge(e0, Math.random()), [x1, y1] = edge(e1, Math.random());
    const len = Math.hypot(x1 - x0, y1 - y0) || 1, dx = (x1 - x0) / len, dy = (y1 - y0) / len;
    // Start and end TRAIL px outside the screen so the trail enters and leaves whole.
    shot = {
      x0: x0 - dx * TRAIL, y0: y0 - dy * TRAIL, dx, dy,
      len: len + 2 * TRAIL, start: now, dur: (len + 2 * TRAIL) / SHOOT_SPEED * 1000,
    };
  }

  function drawShot(now) {
    if (!shot) {
      if (nextShot === 0) nextShot = now + nextGap();
      // Overdue by more than a few seconds means the tab was hidden: reschedule instead of firing on return.
      if (now - nextShot > 5000) nextShot = now + nextGap();
      if (now >= nextShot) { startShot(now); nextShot = now + nextGap(); }
      return;
    }
    const k = (now - shot.start) / shot.dur;
    if (k >= 1) { shot = null; return; }
    const hx = shot.x0 + shot.dx * shot.len * k, hy = shot.y0 + shot.dy * shot.len * k;
    const tail = ctx.createLinearGradient(hx, hy, hx - shot.dx * TRAIL, hy - shot.dy * TRAIL);
    tail.addColorStop(0, fade(pal.trail));
    tail.addColorStop(1, fade(pal.trail, 0));
    ctx.globalAlpha = 1;
    ctx.strokeStyle = tail;
    ctx.lineWidth = 2;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(hx, hy);
    ctx.lineTo(hx - shot.dx * TRAIL, hy - shot.dy * TRAIL);
    ctx.stroke();
    if (pal.halo.a > 0) {
      const g = ctx.createRadialGradient(hx, hy, 0, hx, hy, 9);
      g.addColorStop(0, fade(pal.halo));
      g.addColorStop(1, fade(pal.halo, 0));
      ctx.fillStyle = g;
      ctx.fillRect(hx - 9, hy - 9, 18, 18);
    }
    ctx.fillStyle = pal.head;
    ctx.beginPath();
    ctx.arc(hx, hy, 1.7, 0, 6.2832);
    ctx.fill();
  }

  // Triple click in the bottom-right corner on bare sky: a document listener, not an element,
  // so it can never cover or steal clicks from anything.
  const INTERACTIVE = 'a, button, input, select, textarea, label, summary, details, [role="button"], [tabindex], [class*="overlay"], iframe, img, svg, pre, code, table, kbd, blockquote, hr, .admonition, .highlight';
  const bare = (el) => el instanceof Element && !el.closest(INTERACTIVE) &&
    ![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim());
  const armed = (e) => e.clientX >= innerWidth - CORNER && e.clientY >= innerHeight - CORNER &&
    bare(e.target) && !still.matches && canvas.style.display !== "none";
  // A repeat click selects the nearest text, even from blank space.
  document.addEventListener("mousedown", (e) => { if (e.detail > 1 && armed(e)) e.preventDefault(); });
  let clicks = [];
  document.addEventListener("click", (e) => {
    if (!armed(e)) { clicks = []; return; }
    clicks = clicks.filter((t) => e.timeStamp - t < TRIPLE_MS);
    clicks.push(e.timeStamp);
    if (clicks.length >= 3) { clicks = []; if (!shot) startShot(performance.now()); }
  });

  function mark(shape, x, y, sz) {
    if (shape === "cross") {
      const r = sz * 1.6;
      ctx.lineWidth = 0.8;
      ctx.beginPath();
      ctx.moveTo(x - r, y); ctx.lineTo(x + r, y);
      ctx.moveTo(x, y - r); ctx.lineTo(x, y + r);
      ctx.stroke();
    } else if (shape === "round") {
      ctx.beginPath();
      ctx.arc(x, y, sz / 2 + 0.4, 0, 6.2832);
      ctx.fill();
    } else {
      ctx.fillRect(x, y, sz, sz);
    }
  }

  function draw(now) {
    ctx.clearRect(0, 0, W, H);
    const t = still.matches ? 0 : now / 1000;
    const sy = still.matches ? 0 : scrollY;
    const twinkle = (s, f) => still.matches ? 1 : 0.65 + 0.35 * Math.sin(s.t * 6.2832 + t * 0.9 * TWINKLE * (0.5 + f));
    layers.forEach((L, i) => {
      const front = i === layers.length - 1;
      const shape = front ? pal.bright : pal.shape;
      for (const s of L.stars) {
        const x = mod(s.x * W + t * DRIFT * L.f * 6, W);
        const y = mod(s.y * H - sy * L.f * 0.25, H);
        ctx.globalAlpha = pal.alpha[i] * twinkle(s, L.f);
        ctx.fillStyle = ctx.strokeStyle = pal.stars[s.c];
        mark(shape, x, y, L.sz);
      }
    });

    // Constellations ride with the front layer and wrap as a whole, so no line is ever cut by the screen edge.
    const f = layers[layers.length - 1].f, a = pal.alpha[pal.alpha.length - 1];
    for (const c of sky) {
      const ox = mod(c.ax * W + t * DRIFT * f * 6 + c.w, W + 2 * c.w) - c.w;
      const oy = mod(c.ay * H - sy * f * 0.25 + c.h, H + 2 * c.h) - c.h;
      ctx.globalAlpha = 1;
      ctx.strokeStyle = pal.lines;
      ctx.lineWidth = 1;
      ctx.beginPath();
      c.nodes.forEach((n, k) => { if (k) ctx.lineTo(ox + n.x, oy + n.y); else ctx.moveTo(ox + n.x, oy + n.y); });
      ctx.stroke();
      c.nodes.forEach((n) => {
        ctx.globalAlpha = a * twinkle(n, f);
        ctx.fillStyle = pal.stars[2];
        mark("round", ox + n.x, oy + n.y, DOT);
      });
    }
    ctx.globalAlpha = 1;
  }

  function frame(now) {
    draw(now);
    drawShot(now);
    raf = requestAnimationFrame(frame);
  }

  function sync() {
    cancelAnimationFrame(raf);
    if (!document.body || !document.body.dataset.mdColorScheme) { canvas.style.display = "none"; return; }
    readPalette();
    canvas.style.display = "block";
    draw(performance.now());
    if (!still.matches) raf = requestAnimationFrame(frame);
  }

  addEventListener("resize", resize);
  still.addEventListener("change", sync);
  document.addEventListener("DOMContentLoaded", () => {
    resize();
    sync();
    new MutationObserver(sync).observe(document.body, { attributes: true, attributeFilter: ["data-md-color-scheme"] });
  });
})();
