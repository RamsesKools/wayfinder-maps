// Dark-mode star sky: parallax layers that twinkle and drift very slightly, like the viewer.
(() => {
  const LAYERS = [[0.15, 220, 0.7, 0.5], [0.30, 120, 1.1, 0.7], [0.50, 50, 1.8, 0.95]];
  const tint = ["#ffffff", "#b9c8ff", "#ffe6a0"];
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
  const layers = LAYERS.map(([f, n, sz, a]) => ({
    f, sz, a,
    stars: Array.from({ length: n }, () => ({ x: rnd(), y: rnd(), t: rnd(), c: tint[Math.floor(rnd() * tint.length)] })),
  }));

  let W = 0, H = 0, raf = 0, shot = null, nextShot = 0;
  const mod = (v, m) => ((v % m) + m) % m;

  function resize() {
    const dpr = window.devicePixelRatio || 1;
    W = innerWidth; H = innerHeight;
    canvas.width = W * dpr; canvas.height = H * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw(performance.now());
  }

  // One star crossing the sky from a random edge point to another edge, with a short glowing trail.
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
    tail.addColorStop(0, "rgba(223,232,255,0.6)");
    tail.addColorStop(1, "rgba(223,232,255,0)");
    ctx.globalAlpha = 1;
    ctx.strokeStyle = tail;
    ctx.lineWidth = 2;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(hx, hy);
    ctx.lineTo(hx - shot.dx * TRAIL, hy - shot.dy * TRAIL);
    ctx.stroke();
    const g = ctx.createRadialGradient(hx, hy, 0, hx, hy, 9);
    g.addColorStop(0, "rgba(255,255,255,0.6)");
    g.addColorStop(1, "rgba(255,255,255,0)");
    ctx.globalAlpha = 1;
    ctx.fillStyle = g;
    ctx.fillRect(hx - 9, hy - 9, 18, 18);
    ctx.fillStyle = "#ffffff";
    ctx.beginPath();
    ctx.arc(hx, hy, 1.7, 0, 6.2832);
    ctx.fill();
  }

  // Triple click in the bottom-right corner on bare sky: a document listener, not an element,
  // so it can never cover or steal clicks from anything.
  const INTERACTIVE = 'a, button, input, select, textarea, label, summary, details, [role="button"], [tabindex], [class*="overlay"], iframe, img, svg, pre, code, table, kbd, blockquote, hr, .admonition, .highlight';
  const bare = (el) => el instanceof Element && !el.closest(INTERACTIVE) &&
    ![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim());
  let clicks = [];
  document.addEventListener("click", (e) => {
    const inCorner = e.clientX >= innerWidth - CORNER && e.clientY >= innerHeight - CORNER;
    if (!inCorner || !bare(e.target) || still.matches || canvas.style.display === "none") { clicks = []; return; }
    clicks = clicks.filter((t) => e.timeStamp - t < TRIPLE_MS);
    clicks.push(e.timeStamp);
    if (clicks.length >= 3) { clicks = []; if (!shot) startShot(performance.now()); }
  });

  function draw(now) {
    ctx.clearRect(0, 0, W, H);
    const t = still.matches ? 0 : now / 1000;
    const sy = still.matches ? 0 : scrollY;
    for (const L of layers) {
      for (const s of L.stars) {
        const x = mod(s.x * W + t * DRIFT * L.f * 6, W);
        const y = mod(s.y * H - sy * L.f * 0.25, H);
        const tw = still.matches ? 1 : 0.65 + 0.35 * Math.sin(s.t * 6.2832 + t * 0.9 * TWINKLE * (0.5 + L.f));
        ctx.globalAlpha = L.a * tw;
        ctx.fillStyle = s.c;
        ctx.fillRect(x, y, L.sz, L.sz);
      }
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
    if (!document.body || document.body.dataset.mdColorScheme !== "slate") { canvas.style.display = "none"; return; }
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
