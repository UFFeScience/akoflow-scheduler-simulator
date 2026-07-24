import { mkdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";

const outDir = process.argv[2] || "/tmp/beam-search-gif/frames";

const W = 920;
const H = 900;
const X_SCALE = 0.58;
const X_PAD = 24;

const colors = {
  bg: "#ffffff",
  grid: "#e5e7eb",
  ink: "#111827",
  muted: "#64748b",
  line: "#cbd5e1",
  accent: "#0f62fe",
  accentSoft: "#e8f1ff",
  cost: "#b45309",
  costSoft: "#fef3c7",
  explore: "#7c3aed",
  exploreSoft: "#ede9fe",
  time: "#0f766e",
  timeSoft: "#ccfbf1",
  good: "#166534",
  goodSoft: "#dcfce7",
  drop: "#9ca3af",
  dropSoft: "#f3f4f6",
  danger: "#b91c1c",
};

const nodes = {
  "root-cost": { x: 230, y: 116, title: "Spend less", sub: "wC=1.0", tone: "cost" },
  "root-exp": { x: 740, y: 116, title: "Explore", sub: "9 frontiers", tone: "explore" },
  "root-time": { x: 1250, y: 116, title: "Finish earlier", sub: "wT=1.0", tone: "time" },
  c1: { x: 126, y: 270, title: "C1 edge-1", sub: "S=0.31 T=42 C=8", w: 86 },
  c2: { x: 300, y: 270, title: "C2 edge-2", sub: "S=0.46 T=39 C=10", w: 72 },
  c3: { x: 430, y: 270, title: "C3 cloud-1", sub: "S=0.71 T=31 C=21", w: 48 },
  e1: { x: 560, y: 270, title: "E1 hybrid", sub: "S=0.28 T=35 C=12", w: 92 },
  e2: { x: 740, y: 270, title: "E2 edge+cloud", sub: "S=0.36 T=33 C=15", w: 80 },
  e3: { x: 902, y: 270, title: "E3 duplicate", sub: "S=0.52 T=34 C=15", w: 66 },
  t1: { x: 1084, y: 270, title: "T1 cloud-1", sub: "S=0.29 T=27 C=19", w: 90 },
  t2: { x: 1260, y: 270, title: "T2 cloud-2", sub: "S=0.44 T=25 C=24", w: 74 },
  t3: { x: 1370, y: 270, title: "T3 expensive", sub: "S=0.77 T=24 C=33", w: 42 },
  c1a: { x: 226, y: 434, title: "C1-A", sub: "S=0.62 T=86 C=17", w: 78 },
  c1b: { x: 336, y: 434, title: "C1-B", sub: "S=0.91 T=112 C=16", w: 38 },
  c2a: { x: 446, y: 434, title: "C2-A", sub: "S=0.88 T=76 C=24", w: 44 },
  e1a: { x: 584, y: 434, title: "E1-A", sub: "S=0.54 T=69 C=21", w: 88 },
  e2a: { x: 740, y: 434, title: "E2-A", sub: "S=0.66 T=63 C=25", w: 70 },
  e3a: { x: 850, y: 434, title: "E3-A", sub: "S=0.79 T=67 C=25", w: 50 },
  t1a: { x: 976, y: 434, title: "T1-A", sub: "S=0.58 T=51 C=34", w: 84 },
  t2a: { x: 1114, y: 434, title: "T2-A", sub: "S=0.86 T=49 C=42", w: 46 },
  t3a: { x: 1240, y: 434, title: "T3-A", sub: "S=0.97 T=48 C=52", w: 34 },
  f2: { x: 368, y: 598, title: "Final C", sub: "T=128 C=29", w: 76 },
  f1: { x: 584, y: 598, title: "Final E", sub: "T=103 C=36", w: 92 },
  f4: { x: 722, y: 598, title: "Final D", sub: "T=110 C=40", w: 68 },
  f3: { x: 896, y: 598, title: "Final T", sub: "T=92 C=51", w: 82 },
  f5: { x: 1062, y: 598, title: "Final X", sub: "T=91 C=66", w: 36 },
  r2: { x: 522, y: 780, title: "Rec #2", sub: "budget saver", w: 76 },
  r1: { x: 740, y: 780, title: "Rec #1", sub: "selected", w: 96 },
  r3: { x: 958, y: 780, title: "Rec #3", sub: "faster option", w: 82 },
};

const links = {
  "l-root-cost-c1": ["root-cost", "c1"],
  "l-root-cost-c2": ["root-cost", "c2"],
  "l-root-cost-c3": ["root-cost", "c3"],
  "l-root-exp-e1": ["root-exp", "e1"],
  "l-root-exp-e2": ["root-exp", "e2"],
  "l-root-exp-e3": ["root-exp", "e3"],
  "l-root-time-t1": ["root-time", "t1"],
  "l-root-time-t2": ["root-time", "t2"],
  "l-root-time-t3": ["root-time", "t3"],
  "l-c1-c1a": ["c1", "c1a"],
  "l-c1-c1b": ["c1", "c1b"],
  "l-c2-c2a": ["c2", "c2a"],
  "l-e1-e1a": ["e1", "e1a"],
  "l-e2-e2a": ["e2", "e2a"],
  "l-e3-e3a": ["e3", "e3a"],
  "l-t1-t1a": ["t1", "t1a"],
  "l-t2-t2a": ["t2", "t2a"],
  "l-t3-t3a": ["t3", "t3a"],
  "l-c1a-f2": ["c1a", "f2"],
  "l-e1a-f1": ["e1a", "f1"],
  "l-e2a-f4": ["e2a", "f4"],
  "l-t1a-f3": ["t1a", "f3"],
  "l-t2a-f5": ["t2a", "f5"],
  "l-f2-r2": ["f2", "r2"],
  "l-f1-r1": ["f1", "r1"],
  "l-f3-r3": ["f3", "r3"],
};

const roots = ["root-cost", "root-exp", "root-time"];
const layer1 = ["c1", "c2", "c3", "e1", "e2", "e3", "t1", "t2", "t3"];
const layer2 = ["c1a", "c1b", "c2a", "e1a", "e2a", "e3a", "t1a", "t2a", "t3a"];
const finals = ["f2", "f1", "f4", "f3", "f5"];
const recs = ["r2", "r1", "r3"];
const rootLinks = ["l-root-cost-c1", "l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e1", "l-root-exp-e2", "l-root-exp-e3", "l-root-time-t1", "l-root-time-t2", "l-root-time-t3"];
const layer2Links = ["l-c1-c1a", "l-c1-c1b", "l-c2-c2a", "l-e1-e1a", "l-e2-e2a", "l-e3-e3a", "l-t1-t1a", "l-t2-t2a", "l-t3-t3a"];
const finalLinks = ["l-c1a-f2", "l-e1a-f1", "l-e2a-f4", "l-t1a-f3", "l-t2a-f5"];
const recLinks = ["l-f2-r2", "l-f1-r1", "l-f3-r3"];
const keep1 = ["c1", "e1", "e2", "t1"];
const drop1 = ["c2", "c3", "e3", "t2", "t3"];
const keep2 = ["c1a", "e1a", "e2a", "t1a"];
const drop2 = ["c1b", "c2a", "e3a", "t2a", "t3a"];
const keepFinal = ["f2", "f1", "f4", "f3"];
const dropFinal = ["f5"];

const steps = [
  { title: "1/8  create objective frontiers", gates: [], visible: roots, keep: [], drop: [], linkVisible: [], linkKeep: [], linkDrop: [], selected: [], selectedLinks: [], ranks: [] },
  { title: "2/8  expand candidates for task 1", gates: [], visible: roots.concat(layer1), keep: [], drop: [], linkVisible: rootLinks, linkKeep: [], linkDrop: [], selected: [], selectedLinks: [], ranks: [] },
  { title: "3/8  beam width keeps top states", gates: ["gate-1"], visible: roots.concat(layer1), keep: keep1, drop: drop1, linkVisible: rootLinks, linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3"], selected: [], selectedLinks: [], ranks: [] },
  { title: "4/8  expand only surviving states", gates: ["gate-1"], visible: roots.concat(layer1, layer2), keep: keep1, drop: drop1, linkVisible: rootLinks.concat(layer2Links), linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3"], selected: [], selectedLinks: [], ranks: [] },
  { title: "5/8  prune next layer", gates: ["gate-1", "gate-2"], visible: roots.concat(layer1, layer2), keep: keep1.concat(keep2), drop: drop1.concat(drop2), linkVisible: rootLinks.concat(layer2Links), linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1", "l-c1-c1a", "l-e1-e1a", "l-e2-e2a", "l-t1-t1a"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3", "l-c1-c1b", "l-c2-c2a", "l-e3-e3a", "l-t2-t2a", "l-t3-t3a"], selected: [], selectedLinks: [], ranks: [] },
  { title: "6/8  build final schedules", gates: ["gate-1", "gate-2", "gate-3"], visible: roots.concat(layer1, layer2, finals), keep: keep1.concat(keep2, keepFinal), drop: drop1.concat(drop2, dropFinal), linkVisible: rootLinks.concat(layer2Links, finalLinks), linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1", "l-c1-c1a", "l-e1-e1a", "l-e2-e2a", "l-t1-t1a", "l-c1a-f2", "l-e1a-f1", "l-e2a-f4", "l-t1a-f3"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3", "l-c1-c1b", "l-c2-c2a", "l-e3-e3a", "l-t2-t2a", "l-t3-t3a", "l-t2a-f5"], selected: [], selectedLinks: [], ranks: [] },
  { title: "7/8  rank recommendation items", gates: ["gate-1", "gate-2", "gate-3"], visible: roots.concat(layer1, layer2, finals, recs), keep: keep1.concat(keep2, keepFinal, recs), drop: drop1.concat(drop2, dropFinal), linkVisible: rootLinks.concat(layer2Links, finalLinks, recLinks), linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1", "l-c1-c1a", "l-e1-e1a", "l-e2-e2a", "l-t1-t1a", "l-c1a-f2", "l-e1a-f1", "l-e2a-f4", "l-t1a-f3", "l-f2-r2", "l-f1-r1", "l-f3-r3"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3", "l-c1-c1b", "l-c2-c2a", "l-e3-e3a", "l-t2-t2a", "l-t3-t3a", "l-t2a-f5"], selected: [], selectedLinks: [], ranks: ["r2", "r1", "r3"] },
  { title: "8/8  select Recommendation #1", gates: ["gate-1", "gate-2", "gate-3"], visible: roots.concat(layer1, layer2, finals, recs), keep: keep1.concat(keep2, keepFinal, recs), drop: drop1.concat(drop2, dropFinal), linkVisible: rootLinks.concat(layer2Links, finalLinks, recLinks), linkKeep: ["l-root-cost-c1", "l-root-exp-e1", "l-root-exp-e2", "l-root-time-t1", "l-c1-c1a", "l-e1-e1a", "l-e2-e2a", "l-t1-t1a", "l-c1a-f2", "l-e1a-f1", "l-e2a-f4", "l-t1a-f3", "l-f2-r2", "l-f1-r1", "l-f3-r3"], linkDrop: ["l-root-cost-c2", "l-root-cost-c3", "l-root-exp-e3", "l-root-time-t2", "l-root-time-t3", "l-c1-c1b", "l-c2-c2a", "l-e3-e3a", "l-t2-t2a", "l-t3-t3a", "l-t2a-f5"], selected: ["root-exp", "e1", "e1a", "f1", "r1"], selectedLinks: ["l-root-exp-e1", "l-e1-e1a", "l-e1a-f1", "l-f1-r1"], ranks: ["r2", "r1", "r3"] },
];

function esc(value) {
  return String(value).replace(/[&<>"]/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;" })[char]);
}

function has(step, key, id) {
  return step[key].includes(id);
}

function sx(value) {
  return Math.round(value * X_SCALE + X_PAD);
}

function linkPath(id) {
  const [fromId, toId] = links[id];
  const from = nodes[fromId];
  const to = nodes[toId];
  const y1 = from.y + 34;
  const y2 = to.y - 34;
  const mid = (y1 + y2) / 2;
  return `M${sx(from.x)} ${y1} C${sx(from.x)} ${mid} ${sx(to.x)} ${mid} ${sx(to.x)} ${y2}`;
}

function renderNode(id, step) {
  const node = nodes[id];
  const visible = has(step, "visible", id);
  const selected = has(step, "selected", id);
  const dropped = has(step, "drop", id);
  const kept = has(step, "keep", id);
  const opacity = visible ? (dropped ? 0.36 : 1) : 0.05;
  let fill = "#ffffff";
  let stroke = "#94a3b8";
  if (node.tone === "cost") [fill, stroke] = [colors.costSoft, colors.cost];
  if (node.tone === "explore") [fill, stroke] = [colors.exploreSoft, colors.explore];
  if (node.tone === "time") [fill, stroke] = [colors.timeSoft, colors.time];
  if (kept) [fill, stroke] = [colors.accentSoft, colors.accent];
  if (dropped) [fill, stroke] = [colors.dropSoft, colors.drop];
  if (selected) [fill, stroke] = [colors.goodSoft, colors.good];
  const x = sx(node.x);
  const bar = node.w == null ? "" : `<rect x="${x - 52}" y="${node.y + 18}" width="104" height="6" rx="3" fill="#e2e8f0"/><rect x="${x - 52}" y="${node.y + 18}" width="${node.w}" height="6" rx="3" fill="${selected ? colors.good : dropped ? colors.drop : colors.accent}"/>`;
  const slash = dropped ? `<line x1="${x - 48}" y1="${node.y - 22}" x2="${x + 48}" y2="${node.y + 22}" stroke="${colors.danger}" stroke-width="3"/>` : "";
  return `<g opacity="${opacity}">
    <rect x="${x - 70}" y="${node.y - 34}" width="140" height="68" rx="7" fill="${fill}" stroke="${stroke}" stroke-width="${selected ? 3.5 : 2}"/>
    ${slash}
    <text x="${x}" y="${node.y - 14}" fill="${colors.ink}" font-size="13" font-weight="900" text-anchor="middle">${esc(node.title)}</text>
    <text x="${x}" y="${node.y + 3}" fill="${colors.muted}" font-size="10" font-weight="800" text-anchor="middle">${esc(node.sub)}</text>
    ${bar}
  </g>`;
}

function renderLink(id, step) {
  if (!has(step, "linkVisible", id)) return "";
  const selected = has(step, "selectedLinks", id);
  const dropped = has(step, "linkDrop", id);
  const kept = has(step, "linkKeep", id);
  const stroke = selected ? colors.good : dropped ? colors.drop : kept ? colors.accent : colors.line;
  const width = selected ? 5 : kept ? 3.2 : 2.5;
  const dash = dropped ? ` stroke-dasharray="7 6"` : "";
  const opacity = selected ? 1 : dropped ? 0.4 : kept ? 1 : 0.58;
  return `<path d="${linkPath(id)}" fill="none" stroke="${stroke}" stroke-width="${width}" opacity="${opacity}"${dash}/>`;
}

function renderGate(id, step) {
  if (!has(step, "gates", id)) return "";
  const attrs = {
    "gate-1": [66, 236, 1348, 94],
    "gate-2": [120, 400, 1240, 94],
    "gate-3": [252, 564, 976, 94],
  }[id];
  return `<rect x="${sx(attrs[0])}" y="${attrs[1]}" width="${Math.round(attrs[2] * X_SCALE)}" height="${attrs[3]}" rx="14" fill="rgba(15,98,254,0.055)" stroke="${colors.accent}" stroke-width="2" stroke-dasharray="9 7"/>`;
}

function renderRank(id, step) {
  if (!has(step, "ranks", id)) return "";
  const node = nodes[id];
  const x = sx(node.x);
  const label = id === "r1" ? "recommended" : id === "r2" ? "rank 2" : "rank 3";
  const width = id === "r1" ? 124 : 108;
  return `<g><rect x="${x - width / 2}" y="828" width="${width}" height="28" rx="14" fill="#ffffff" stroke="${colors.good}" stroke-width="2"/><text x="${x}" y="846" fill="${colors.good}" font-size="12" font-weight="900" text-anchor="middle">${label}</text></g>`;
}

function frameSvg(step) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="${colors.bg}"/>
  <defs><pattern id="grid" width="32" height="32" patternUnits="userSpaceOnUse"><circle cx="24" cy="24" r="1" fill="${colors.grid}"/></pattern></defs>
  <rect width="${W}" height="${H}" fill="url(#grid)" opacity="0.7"/>
  <text x="28" y="43" fill="${colors.accent}" font-size="12" font-weight="900">${esc(step.title.split("  ")[0])}</text>
  <text x="76" y="44" fill="${colors.ink}" font-size="20" font-weight="900">${esc(step.title.split("  ")[1])}</text>
  <text x="${sx(740)}" y="96" class="layer" fill="${colors.muted}" font-size="12" font-weight="900" text-anchor="middle">OBJECTIVE FRONTIERS</text>
  <text x="${sx(740)}" y="218" fill="${colors.muted}" font-size="12" font-weight="900" text-anchor="middle">EXPAND TASK 1</text>
  <text x="${sx(740)}" y="382" fill="${colors.muted}" font-size="12" font-weight="900" text-anchor="middle">EXPAND TASK 2</text>
  <text x="${sx(740)}" y="546" fill="${colors.muted}" font-size="12" font-weight="900" text-anchor="middle">FINAL STATES</text>
  <text x="${sx(740)}" y="728" fill="${colors.muted}" font-size="12" font-weight="900" text-anchor="middle">RECOMMENDATIONS</text>
  ${["gate-1", "gate-2", "gate-3"].map((id) => renderGate(id, step)).join("")}
  ${Object.keys(links).map((id) => renderLink(id, step)).join("")}
  ${Object.keys(nodes).map((id) => renderNode(id, step)).join("")}
  ${recs.map((id) => renderRank(id, step)).join("")}
  </svg>`;
}

await rm(outDir, { recursive: true, force: true });
await mkdir(outDir, { recursive: true });

let frame = 1;
for (const step of steps) {
  for (let hold = 0; hold < 12; hold += 1) {
    await writeFile(path.join(outDir, `frame-${String(frame).padStart(3, "0")}.svg`), frameSvg(step));
    frame += 1;
  }
}

console.log(`wrote ${frame - 1} SVG frames to ${outDir}`);
