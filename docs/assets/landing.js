(() => {
  const root = document.querySelector(".story");
  if (!root) return;
  const chapters = [...root.querySelectorAll(".story__chapter")];
  const NS = "http://www.w3.org/2000/svg";
  const still = matchMedia("(prefers-reduced-motion: reduce)").matches;

  const COL = {
    decision: {
      resolved: { core: "#b9d6c4", glow: "#5b9077", r: 5.4, gr: 24 },
      frontier: { core: "#8ad8ff", glow: "#2f9be0", r: 8.1, gr: 46 },
      blocked: { core: "#e2c3c3", glow: "#9a6f6f", r: 4.5, gr: 20 },
    },
    implement: {
      resolved: { core: "#a4d9cb", glow: "#2f8f7c", r: 6.2, gr: 25 },
      frontier: { core: "#86f5d2", glow: "#12b88f", r: 9.2, gr: 46 },
      blocked: { core: "#e2c3c3", glow: "#9a6f6f", r: 5.2, gr: 21 },
    },
  };
  const LABEL = { resolved: "#a2c1ac", frontier: "#b3e5ff", blocked: "#d0b3b3" };
  const STATION = "#5fd1c0";
  const WHO = {
    afk: { color: "#8fe0cf", d: "M4.5 0.4h1v2.6h-1zM2 3h6a1.4 1.4 0 0 1 1.4 1.4v3.6a1.4 1.4 0 0 1-1.4 1.4h-6a1.4 1.4 0 0 1-1.4-1.4v-3.6a1.4 1.4 0 0 1 1.4-1.4zM3.1 5.2a0.9 0.9 0 1 0 1.8 0a0.9 0.9 0 1 0-1.8 0zM5.1 5.2a0.9 0.9 0 1 0 1.8 0a0.9 0.9 0 1 0-1.8 0z" },
    hitl: { color: "#e6dcc8", d: "M2.8 2.8a2.2 2.2 0 1 0 4.4 0a2.2 2.2 0 1 0-4.4 0zM1 9.6c0-2.8 1.7-4.2 4-4.2s4 1.4 4 4.2z" },
  };

  const NODES = [
    { id: "s", kind: "start", x: 46, y: 195, on: 0 },
    { id: "g", kind: "goal", x: 540, y: 195, on: 0, done: 4 },
    { id: "d1", kind: "decision", num: "01", x: 136, y: 112, on: 1, done: 2, label: "Audience", who: "hitl" },
    { id: "d2", kind: "decision", num: "02", x: 136, y: 278, on: 1, done: 2, label: "Format", who: "afk" },
    { id: "d3", kind: "decision", num: "03", x: 240, y: 195, on: 1, done: 2, label: "Sources", who: "afk" },
    { id: "d4", kind: "decision", num: "04", x: 340, y: 112, on: 1, done: 3, label: "Grouping", who: "afk" },
    { id: "d5", kind: "decision", num: "05", x: 340, y: 278, on: 1, done: 3, label: "Hosting", who: "hitl" },
    { id: "i1", kind: "implement", num: "06", x: 436, y: 140, on: 3, done: 3, label: "Parse PRs", who: "afk" },
    { id: "i2", kind: "implement", num: "07", x: 436, y: 252, on: 3, done: 4, label: "Render", who: "afk" },
    { id: "i3", kind: "implement", num: "08", x: 486, y: 195, on: 3, done: 4, label: "Publish", who: "afk" },
  ];
  const EDGES = [
    ["s", "d1"], ["s", "d2"], ["d1", "d3"], ["d2", "d3"], ["d3", "d4"], ["d3", "d5"],
    ["d4", "i1"], ["d5", "i2"], ["i1", "i3"], ["i2", "i3"], ["i3", "g"],
  ];
  const FOG = [1, 0.8, 0.55, 0.3, 0];
  const byId = Object.fromEntries(NODES.map((n) => [n.id, n]));
  const work = NODES.filter((n) => n.kind === "decision" || n.kind === "implement");

  const resolved = (n, step) => n.kind === "start" || step >= n.done;
  const statusOf = (n, step) => {
    if (step >= n.done) return "resolved";
    const blockers = EDGES.filter((e) => e[1] === n.id).map((e) => byId[e[0]]);
    return blockers.every((b) => resolved(b, step)) ? "frontier" : "blocked";
  };

  const el = (name, attrs = {}, parent) => {
    const e = document.createElementNS(NS, name);
    for (const k in attrs) e.setAttribute(k, attrs[k]);
    if (parent) parent.appendChild(e);
    return e;
  };
  let uidN = 0;
  const build = (stage, onDot) => {
    const uid = `sm${uidN++}`;
    const gradient = (defs, id, stops) => {
      const g = el("radialGradient", { id: `${uid}-${id}` }, defs);
      stops.forEach(([o, c, a]) => el("stop", { offset: o, "stop-color": c, "stop-opacity": a }, g));
    };
    const svg = el("svg", { class: "story__svg", viewBox: "0 0 600 420", role: "presentation" });
    const defs = el("defs", {}, svg);
    for (const kind in COL) {
      for (const st in COL[kind]) {
        const c = COL[kind][st];
        gradient(defs, `sg-${kind}-${st}`, [["0", c.glow, 0.85], ["0.4", c.glow, 0.22], ["1", c.glow, 0]]);
        gradient(defs, `sc-${kind}-${st}`, [["0", c.core, 1], ["0.6", c.core, 0.92], ["0.82", c.core, 0.45], ["1", c.core, 0]]);
        gradient(defs, `ss-${kind}-${st}`, [["0", c.core, 1], ["0.65", c.core, 0.94], ["1", c.glow, 0.95]]);
      }
    }
    gradient(defs, "sg-start", [["0", "#9cc4ff", 0.7], ["0.4", "#6a9be8", 0.22], ["1", "#6a9be8", 0]]);
    gradient(defs, "sg-goal", [["0", "#ffb020", 0.8], ["0.45", "#ffb020", 0.25], ["1", "#ffb020", 0]]);
    gradient(defs, "sf-a", [["0", "#7a66b0", 0.55], ["1", "#7a66b0", 0]]);
    gradient(defs, "sf-b", [["0", "#c89b3c", 0.4], ["1", "#c89b3c", 0]]);
    gradient(defs, "sf-c", [["0", "#4f6aa8", 0.5], ["1", "#4f6aa8", 0]]);

    const sky = el("g", { class: "ssky" }, svg);
    let seed = 9001;
    const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);
    for (let i = 0; i < 110; i++) {
      const c = el("circle", { cx: (rnd() * 600).toFixed(1), cy: (rnd() * 420).toFixed(1), r: (0.5 + rnd() * 0.9).toFixed(2), opacity: (0.25 + rnd() * 0.6).toFixed(2) }, sky);
      if (!still && i % 4 === 0) c.style.animationDelay = `${(rnd() * 4).toFixed(1)}s`, c.classList.add("tw");
    }

    const fog = el("g", { class: "sfog" }, svg);
    const puffs = [
      [430, 195, 130, "sf-a"], [350, 135, 90, "sf-c"], [350, 255, 95, "sf-a"], [500, 105, 85, "sf-b"],
      [500, 290, 90, "sf-c"], [250, 195, 75, "sf-a"], [545, 195, 70, "sf-b"],
    ];
    puffs.forEach(([x, y, r, g], i) => {
      const c = el("circle", { cx: x, cy: y, r, fill: `url(#${uid}-${g})`, class: "sfog__puff" }, fog);
      c.style.setProperty("--d", `${i * 1.3}s`);
    });
    [[440, 160, "Translations?"], [505, 312, "Release automation?"], [330, 195, "Truncation rules?"]].forEach(([x, y, t]) => {
      el("text", { x, y, class: "sfog__t", "text-anchor": "middle" }, fog).textContent = t;
    });

    const edgeLayer = el("g", {}, svg);
    const nodeLayer = el("g", {}, svg);
    const edges = EDGES.map(([a, b], i) => {
      const p = byId[a], q = byId[b];
      const mx = (p.x + q.x) / 2, my = (p.y + q.y) / 2;
      const dx = q.x - p.x, dy = q.y - p.y, len = Math.hypot(dx, dy) || 1;
      const bow = Math.min(16, len * 0.13), nx = -dy / len, ny = dx / len;
      const cx = mx + nx * bow, cy = my + ny * bow;
      const d = `M${p.x} ${p.y}Q${cx} ${cy} ${q.x} ${q.y}`;
      const g = el("g", { class: "se" }, edgeLayer);
      el("path", { d, class: "se__line" }, g);
      const ax = 0.25 * p.x + 0.5 * cx + 0.25 * q.x, ay = 0.25 * p.y + 0.5 * cy + 0.25 * q.y;
      const ux = dx / len, uy = dy / len, px = -uy, py = ux, ah = 7, aw = 3.8;
      const tx = ax + ux * ah * 0.5, ty = ay + uy * ah * 0.5;
      el("polygon", { class: "se__arrow", points: `${tx},${ty} ${tx - ux * ah + px * aw},${ty - uy * ah + py * aw} ${tx - ux * ah - px * aw},${ty - uy * ah - py * aw}` }, g);
      if (!still) {
        const m = el("circle", { r: 1.7, class: "se__mote" }, g);
        el("animateMotion", { dur: `${6 + (i % 3)}s`, repeatCount: "indefinite", path: d, begin: `${(i * 0.7).toFixed(1)}s` }, m);
      }
      return { g, from: p, to: q, on: Math.max(p.on, q.on) };
    });

    const glyph = (parent, who) => {
      const w = WHO[who];
      const g = el("g", { class: "sn__who" }, parent);
      el("circle", { r: 7.6, fill: "rgba(5,7,13,0.8)", stroke: w.color, "stroke-opacity": 0.5 }, g);
      el("path", { d: w.d, transform: "translate(-5 -5)", fill: w.color, "fill-rule": "evenodd" }, g);
      return g;
    };

    const nodes = NODES.map((n) => {
      const g = el("g", { class: `sn sn--${n.kind}`, transform: `translate(${n.x} ${n.y})` }, nodeLayer);
      const rec = { n, g };
      if (n.kind === "start") {
        el("circle", { r: 26, fill: `url(#${uid}-sg-start)` }, g);
        el("circle", { r: 4.5, fill: "#e8f0ff" }, g);
        el("text", { class: "sn__tag", y: 26, "text-anchor": "middle" }, g).textContent = "START";
      } else if (n.kind === "goal") {
        rec.glow = el("circle", { r: 54, fill: `url(#${uid}-sg-goal)`, class: "sn__goalglow" }, g);
        el("circle", { r: 5, fill: "#ffe9b0" }, g);
        el("text", { class: "sn__tag sn__tag--goal", y: 54, "text-anchor": "middle" }, g).textContent = "GOAL: Release notes";
      } else {
        rec.flare = el("circle", { class: "sn__flare", r: 8 }, g);
        rec.glow = el("circle", {}, g);
        rec.core = n.kind === "implement" ? el("polygon", {}, g) : el("circle", {}, g);
        rec.ring = n.kind === "implement" ? el("polygon", { class: "sn__ring" }, g) : null;
        rec.label = el("text", { class: "sn__label", "text-anchor": "middle" }, g);
        rec.label.textContent = `${n.num}  ${n.label}`;
        rec.who = glyph(g, n.who);
      }
      return rec;
    });

    const hexPoints = (R) => Array.from({ length: 6 }, (_, k) => {
      const a = -Math.PI / 2 + (k * Math.PI) / 3;
      return `${(Math.cos(a) * R).toFixed(2)},${(Math.sin(a) * R).toFixed(2)}`;
    }).join(" ");

    const paint = (rec, st) => {
      const { n } = rec, c = COL[n.kind][st];
      rec.glow.setAttribute("r", c.gr);
      rec.glow.setAttribute("fill", `url(#${uid}-sg-${n.kind}-${st})`);
      if (n.kind === "implement") {
        const R = c.r * 1.15;
        rec.core.setAttribute("points", hexPoints(R));
        rec.core.setAttribute("fill", `url(#${uid}-ss-implement-${st})`);
        rec.ring.setAttribute("points", hexPoints(R + 4));
        rec.ring.style.opacity = st === "frontier" ? 0.7 : 0.4;
      } else {
        rec.core.setAttribute("r", c.r * 1.35);
        rec.core.setAttribute("fill", `url(#${uid}-sc-decision-${st})`);
      }
      rec.label.setAttribute("y", c.r + 16);
      rec.label.setAttribute("fill", LABEL[st]);
      rec.who.setAttribute("transform", `translate(${c.r + 9} ${-c.r - 9})`);
      rec.who.style.opacity = st === "frontier" ? 1 : 0;
    };

    const hud = document.createElement("div");
    hud.className = "story__hud";
    const pills = document.createElement("div");
    pills.className = "hud__pills";
    const bar = document.createElement("div");
    bar.className = "hud__bar";
    const fill = document.createElement("i");
    bar.appendChild(fill);
    hud.append(pills, bar);

    const dots = document.createElement("div");
    dots.className = "story__dots";
    const dotBtns = chapters.map((_, i) => {
      const b = document.createElement("button");
      b.type = "button";
      b.setAttribute("aria-label", `Chapter ${i + 1}`);
      b.addEventListener("click", () => onDot(i));
      dots.appendChild(b);
      return b;
    });
    stage.append(svg, dots);
    stage.after(hud);
    const pill = (text, who) => {
      const s = document.createElement("span");
      s.className = "hud__pill";
      if (who) {
        const ic = el("svg", { viewBox: "0 0 10 10", width: 10, height: 10, class: "hud__ic" });
        el("path", { d: WHO[who].d, fill: WHO[who].color, "fill-rule": "evenodd" }, ic);
        s.appendChild(ic);
      }
      s.append(text);
      pills.appendChild(s);
    };

    const prev = new Map();
    let current = -1;
    const setStep = (n) => {
      if (n === current) return;
      const first = current < 0;
      current = n;

      for (const rec of nodes) {
        const on = n >= rec.n.on;
        rec.g.classList.toggle("is-on", on);
        if (rec.n.kind === "goal") rec.g.classList.toggle("is-done", n >= rec.n.done);
        if (!rec.core) continue;
        const st = statusOf(rec.n, n);
        rec.g.classList.remove("st-resolved", "st-frontier", "st-blocked");
        rec.g.classList.add(`st-${st}`);
        paint(rec, st);
        if (!first && prev.get(rec.n.id) && prev.get(rec.n.id) !== st && on) {
          rec.flare.classList.remove("is-flaring");
          void rec.flare.getBoundingClientRect();
          rec.flare.classList.add("is-flaring");
        }
        prev.set(rec.n.id, st);
      }
      for (const e of edges) {
        e.g.classList.toggle("is-on", n >= e.on);
        e.g.classList.toggle("is-sat", resolved(e.from, n));
      }

      fog.style.setProperty("--fog", String(FOG[n] ?? 0));

      const done = work.filter((w) => n >= w.done);
      const dec = work.filter((w) => w.kind === "decision");
      const imp = work.filter((w) => w.kind === "implement");
      const front = work.filter((w) => n >= w.on && statusOf(w, n) === "frontier");
      const afk = front.filter((w) => w.who === "afk").length, hum = front.length - afk;
      pills.textContent = "";
      if (n === 0) {
        pill("destination known");
        pill("route unknown");
      } else {
        if (afk) pill(`${afk} can take`, "afk");
        if (hum) pill(`${hum} need${hum === 1 ? "s" : ""} you`, "hitl");
        pill(`decisions ${dec.filter((w) => n >= w.done).length}/${dec.length}`);
        if (n >= 3) pill(`implement ${imp.filter((w) => n >= w.done).length}/${imp.length}`);
      }
      fill.style.width = `${(done.length / work.length) * 100}%`;

      dotBtns.forEach((b, i) => b.classList.toggle("is-active", i === n));
    };
    return { setStep, destroy: () => { svg.remove(); dots.remove(); hud.remove(); } };
  };

  const narrow = matchMedia("(max-width: 44.9375em)");
  const pin = root.querySelector(".story__pin");
  const stage = root.querySelector(".story__stage");
  let built = [];

  const jump = (i) => chapters[i].scrollIntoView({ behavior: "smooth", block: narrow.matches ? "start" : "center" });

  let wideMap = null;
  const activate = (n) => {
    if (!wideMap) return;
    root.dataset.step = String(n);
    wideMap.setStep(n);
    chapters.forEach((c, i) => c.classList.toggle("is-active", i === n));
  };

  // The active step is the last one whose heading has scrolled above 60% of the viewport.
  const sync = () => {
    const line = window.innerHeight * 0.6;
    activate(Math.max(0, chapters.findLastIndex((c) => c.querySelector("h3").getBoundingClientRect().top < line)));
  };

  const layout = () => {
    built.forEach((b) => b.destroy());
    built = [];
    root.querySelectorAll(".story__card").forEach((c) => c.remove());
    wideMap = null;
    if (narrow.matches) {
      chapters.forEach((c, i) => {
        const card = document.createElement("div");
        card.className = "story__card";
        const cardStage = document.createElement("div");
        cardStage.className = "story__stage";
        card.appendChild(cardStage);
        c.querySelector("h3").after(card);
        const m = build(cardStage, jump);
        m.setStep(i);
        built.push(m);
      });
    } else {
      wideMap = build(stage, jump);
      built.push(wideMap);
      sync();
    }
  };

  root.classList.add("story--live");
  layout();
  narrow.addEventListener("change", layout);
  addEventListener("scroll", sync, { passive: true });
  addEventListener("resize", sync);
})();
