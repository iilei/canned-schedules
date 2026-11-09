// canned-schedules preflight: runs the Go pipeline (compiled to WebAssembly) on every edit. Nothing leaves the page.
"use strict";

const $ = (id) => document.getElementById(id);
const editor = createEditor($("editor"));
const jsonView = createJsonView($("json"));
const DEFAULT_PRESET = "office-hours";
const state = { theme: document.documentElement.dataset.theme || "system", flavour: "sphinx", tab: "rendered", svg: "", myst: "", text: "", preset: "" };
const presets = []; // [{id, label, text}], from presets/index.json

// Content hashes of the files this script fetches, which web:build writes into index.html (editor/stamp-assets.mjs),
// so a deploy never pairs this script with another build's checker or presets. The raw web/ folder has none.
const ASSET_VERSIONS = (() => {
  try { return JSON.parse(document.getElementById("asset-versions")?.textContent || "{}"); } catch (_) { return {}; }
})();
const asset = (rel) => (ASSET_VERSIONS[rel] ? `${rel}?v=${ASSET_VERSIONS[rel]}` : rel);
let current = null; // the selected preset
let ready = false;

// ---- editor ------------------------------------------------------------------------------------------------------

// The editor is CodeMirror (editor.js) with Markdown highlighting and diagnostics marked in place. Without editor.js
// the plain textarea stays, behind the same small interface: value, cursorLine, hasFocus, onChange, onCursor, focus, selectAll,
// jumpTo, setDiagnostics, setHighlight.
function createEditor(textarea) {
  const listeners = [];
  const cursorListeners = [];
  const changed = () => { for (const fn of listeners) fn(); };
  let lastLine = 0;
  const moved = (line) => {
    if (line === lastLine) return;
    lastLine = line;
    for (const fn of cursorListeners) fn(line);
  };

  if (globalThis.CannedEditor) {
    const host = Object.assign(document.createElement("div"), { className: "cm-host editor" });
    textarea.replaceWith(host);
    const cm = globalThis.CannedEditor.createEditor(host, { label: textarea.getAttribute("aria-label"), onChange: changed, onCursor: moved });
    cm.onChange = (fn) => listeners.push(fn);
    cm.onCursor = (fn) => cursorListeners.push(fn);
    return cm;
  }

  const textareaLine = () => textarea.value.slice(0, textarea.selectionStart).split("\n").length;
  textarea.addEventListener("input", changed);
  for (const ev of ["input", "keyup", "click", "select"]) textarea.addEventListener(ev, () => moved(textareaLine()));
  return {
    get value() { return textarea.value; },
    set value(text) { textarea.value = text; },
    get cursorLine() { return textareaLine(); },
    get hasFocus() { return document.activeElement === textarea; },
    onChange: (fn) => listeners.push(fn),
    onCursor: (fn) => cursorListeners.push(fn),
    focus: () => textarea.focus(),
    selectAll: () => textarea.select(),
    setDiagnostics() {},
    setHighlight() {},
    // Columns from the tool count characters (code points); textarea offsets count UTF-16 units.
    jumpTo(line, column) {
      const lines = textarea.value.split("\n");
      let offset = 0;
      for (let i = 0; i < line - 1 && i < lines.length; i++) offset += lines[i].length + 1;
      offset += Array.from(lines[line - 1] || "").slice(0, column - 1).join("").length;
      textarea.focus();
      textarea.setSelectionRange(offset, offset + 1);
      const lineHeight = parseFloat(getComputedStyle(textarea).lineHeight) || 20;
      textarea.scrollTop = Math.max(0, (line - 4) * lineHeight);
    },
  };
}

// The JSON tab: a read-only CodeMirror view with highlighting and folding, or the plain <pre>.
function createJsonView(pre) {
  if (!globalThis.CannedEditor) return { set value(text) { pre.textContent = text; }, setHighlight() {} };
  const host = Object.assign(document.createElement("div"), { className: "cm-host viewer", id: "json" });
  pre.replaceWith(host);
  return globalThis.CannedEditor.createViewer(host, { label: "Canonical JSON output" });
}

// ---- source highlight ----------------------------------------------------------------------------------------------

// The entry under the editor cursor shows where it lands in the outputs. In the SVG week: dots where a
// range or the base paints (dense where its state shows, sparse where a later range paints over it), and
// a frame around the name, the summary or an annotation row. In the JSON tab: the lines it produces. The checker
// computes all of it (rectangles in the preview's buckets, JSON line spans); the overlay lives only in the page's
// DOM, so state.svg, which Download SVG saves, never carries it.
const SVG_NS = "http://www.w3.org/2000/svg";

// The entries on this line (the META marker has its own entry for the whole block); on a line without one (a STATE
// marker, a blank line inside a block), every entry of the block around it.
// Each comes with the source lines to mark in the editor: the whole block for the META marker and for a block
// matched as a whole, otherwise the entry's own line.
function highlightsAt(line) {
  const all = state.highlights || [];
  const hit = all.filter((h) => h.line === line);
  if (hit.length) {
    return hit.map((h) => ({ ...h, source: h.kind === "meta-block" ? h.block : { from: h.line, to: h.line } }));
  }
  return all.filter((h) => h.block.from > 0 && line >= h.block.from && line <= h.block.to)
    .map((h) => ({ ...h, source: h.block }));
}

function svgEl(name, attrs, ...children) {
  const el = document.createElementNS(SVG_NS, name);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  el.append(...children);
  return el;
}

// The overlay draws in black and white side by side, never in a theme or state colour: every colour contrasts at
// least 4.58:1 with black or with white, so one of the two always stands out, whatever the palette, theme or
// override. TestHighlightInkContrast (internal/web) reads these two values and checks every palette's fills.
const INK = { dark: "#000000", light: "#ffffff" };

// Diagonal stripes over painted areas, tilted by 45°, in one ink per entry: black or white, whichever contrasts more
// with the entry's own state colour (contrastInk). Areas where a later range paints over the entry keep that ink,
// whatever the fill there, so one entry always reads in one ink. Each stripe is a solid core fading to transparent at
// its edges, and the whole overlay is half transparent (STRIPE_OPACITY), so it blends into the state colour, and most
// of the area keeps that colour. `period` is the distance between two stripes: dense
// where the entry's state shows, wider where a later range paints over it.
const STRIPE_WIDTH = 2.5;
const STRIPE_OPACITY = 0.5;
const INKS = ["light", "dark"];

function fade(ink) {
  const stop = (offset, opacity) => svgEl("stop", { offset, "stop-color": INK[ink], "stop-opacity": opacity });
  return svgEl("linearGradient", { id: `cs-hl-fade-${ink}` },
    stop(0, 0), stop(0.3, STRIPE_OPACITY), stop(0.7, STRIPE_OPACITY), stop(1, 0));
}

function stripes(density, ink, period) {
  return svgEl("pattern", {
    id: `cs-hl-${density}-${ink}`, width: period, height: period, patternUnits: "userSpaceOnUse", patternTransform: "rotate(45)",
  }, svgEl("rect", {
    x: (period - STRIPE_WIDTH) / 2, y: 0, width: STRIPE_WIDTH, height: period, class: `cs-hl-stripe cs-hl-stripe-${ink}`,
  }));
}

// The fill as drawn: the browser's computed colour (the SVG is rendered in the page's theme, so palettes, overrides and
// the theme are all resolved), blended over the SVG's background. Returns [r, g, b] in 0..255, or null.
function drawnColor(el, under) {
  const m = getComputedStyle(el).fill.match(/rgba?\(([^)]+)\)/);
  if (!m) return null;
  const [r, g, b, a = 1] = m[1].split(/[\s,/]+/).filter(Boolean).map(Number);
  return under ? [r, g, b].map((c, i) => c * a + under[i] * (1 - a)) : [r, g, b];
}

// WCAG relative luminance and contrast ratio.
function luminance(rgb) {
  const [r, g, b] = rgb.map((c) => { const v = c / 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
const contrast = (a, b) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

// Black or white for an entry: the ink with the higher contrast against the drawn colour of its own state (any
// element with the state's class, e.g. a segment or the legend swatch). TestHighlightInkContrast (internal/web)
// checks that the better ink reaches 4.5:1 on every palette fill.
function contrastInk(svg, stateClass) {
  const bgEl = svg.querySelector(".background");
  const bg = (bgEl && drawnColor(bgEl)) || [255, 255, 255];
  const el = stateClass && svg.querySelector(`rect.${stateClass}`);
  const fill = el ? drawnColor(el, bg) : bg;
  if (!fill) return "dark";
  const l = luminance(fill);
  const [lDark, lLight] = [INK.dark, INK.light].map((hex) => luminance([1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))));
  return contrast(l, lDark) >= contrast(l, lLight) ? "dark" : "light";
}

// While the JSON tab is hidden and marks lines for the cursor, its tab glows where the selected tab shows its
// underline: a pulse when the marked lines change, then a soft steady glow until the tab is opened or the marks go.
// The glow waits for the user: the cursor starts on the H1, whose marks aren't news after a page load.
let lastMarks = "";
let cursorUsed = false;

function signalJsonMarks(spans) {
  const tab = document.querySelector('.tabs [data-tab="json"]');
  if (!tab || !cursorUsed) return;
  const key = JSON.stringify(spans);
  const fresh = spans.length > 0 && key !== lastMarks;
  lastMarks = spans.length ? key : "";
  const hidden = state.tab !== "json";
  tab.classList.toggle("has-marks", spans.length > 0 && hidden);
  tab.title = spans.length && hidden ? "Lines for the cursor's entry are marked in the JSON" : "";
  if (fresh && hidden) {
    tab.classList.remove("pulse");
    void tab.offsetWidth; // Restart the animation.
    tab.classList.add("pulse");
  }
}

// The status bar under the editor explains the entry under the cursor. Its height is fixed at two lines, so the page
// never moves while the cursor does: each entry takes one line (cut off with "…", the full text on hover), and a
// third entry or more turns the second line into "+N more". Screen readers get every label.
const STATUS_HINT = "Put the cursor in the schedule to see what each line produces.";

function showSourceStatus(labels) {
  const bar = $("highlight-status");
  const row = (text, title = text) => Object.assign(document.createElement("div"), { textContent: text, title });
  bar.classList.toggle("empty", labels.length === 0);
  if (!labels.length) {
    bar.replaceChildren(row(STATUS_HINT, ""));
    return;
  }
  const rows = labels.length <= 2 ? labels.map((l) => row(l))
    : [row(labels[0]), row(`+${labels.length - 1} more`, labels.slice(1).join("\n"))];
  bar.replaceChildren(...rows);
  bar.setAttribute("aria-label", labels.join(". "));
}

// The overlay draws on top of the week, so the segments under it would show nothing in devtools. Every segment an
// overlay rect covers (even partly) therefore gets `cs-hl-under` plus the density and source line of what covers it,
// e.g. `cs-hl-under cs-hl-under-dense cs-hl-under-line-20`; the next highlight removes them. DOM only: state.svg,
// which Download SVG saves, is the checker's own text.
function markSegments(svg, r, classes) {
  for (const el of svg.querySelectorAll("rect.state")) {
    const x = +el.getAttribute("x"), y = +el.getAttribute("y");
    const w = +el.getAttribute("width"), hgt = +el.getAttribute("height");
    if (r.x < x + w && x < r.x + r.width && r.y < y + hgt && y < r.y + r.height) el.classList.add("cs-hl-under", ...classes);
  }
}

function unmarkSegments(svg) {
  for (const el of svg.querySelectorAll(".cs-hl-under")) {
    el.classList.remove(...[...el.classList].filter((c) => c.startsWith("cs-hl-under")));
  }
}

function applyHighlight(line) {
  const svg = $("svg").querySelector("svg");
  const hs = svg ? highlightsAt(line) : [];
  showSourceStatus(hs.map((h) => h.label));
  editor.setHighlight(hs.map((h) => h.source));
  const spans = hs.flatMap((h) => h.json || []).sort((a, b) => a.from - b.from);
  jsonView.setHighlight(spans);
  signalJsonMarks(spans);
  if (!svg) return;
  svg.querySelector(".cs-highlight")?.remove();
  unmarkSegments(svg);
  if (!hs.length) return;

  const g = svgEl("g", { class: "cs-highlight", "aria-hidden": "true" },
    svgEl("defs", {}, ...INKS.map(fade), ...INKS.flatMap((ink) => [stripes("dense", ink, 7), stripes("sparse", ink, 13)])));
  const at = (r) => ({ x: r.x, y: r.y, width: r.width, height: r.height });
  // Painted areas get only the stripes, no outline. Text has no fill to stripe, so it gets a light frame: one dashed line
  // whose dashes alternate between the light and the dark theme's text colour (style.css), visible on either.
  // Classes say what each rect is (for devtools): kind, source line, state, density and ink. The fill comes from
  // style.css by density and ink, so the stylesheet shows why a rect looks the way it does.
  const area = (r, h, density, ink) => {
    g.append(svgEl("rect", {
      ...at(r), class: `cs-hl-area cs-hl-${h.kind} cs-hl-line-${h.line} cs-hl-${h.class} cs-hl-${density} cs-hl-ink-${ink}`,
    }));
    markSegments(svg, r, [`cs-hl-under-${density}`, `cs-hl-under-line-${h.line}`]);
  };
  const frame = (r, h) => {
    const what = `cs-hl-${h.kind} cs-hl-line-${h.line}${h.key ? ` cs-hl-key-${h.key}` : ""}`;
    g.append(
      svgEl("rect", { ...at(r), rx: 2, class: `cs-hl-frame cs-hl-frame-a ${what}` }),
      svgEl("rect", { ...at(r), rx: 2, class: `cs-hl-frame cs-hl-frame-b ${what}` }));
  };
  const painted = (h) => h.kind === "base" || h.kind === "range";
  const inks = new Map(hs.filter(painted).map((h) => [h, contrastInk(svg, h.class)]));
  for (const h of hs.filter(painted)) for (const r of h.shadowed) area(r, h, "sparse", inks.get(h));
  for (const h of hs) for (const r of h.rects) painted(h) ? area(r, h, "dense", inks.get(h)) : frame(r, h);
  svg.append(g); // Last, so frames around text sit on top; pointer-events: none keeps the tooltips reachable.
}

// ---- theme ---------------------------------------------------------------------------------------------------------

function setTheme(theme) {
  state.theme = theme;
  document.documentElement.dataset.theme = theme;
  try { localStorage.setItem("theme", theme); } catch (_) {}
  pressed($("theme"), theme);
  evaluate(); // The SVG is rendered in the page's theme (--svg-theme).
}

function pressed(group, value) {
  for (const b of group.querySelectorAll("button")) b.setAttribute("aria-pressed", String(b.dataset.value === value));
}

// ---- evaluation ----------------------------------------------------------------------------------------------------

// Flags from the link (?arg=, repeatable, one argument each): the ones the page has no control for, such as
// --error-unless-alert-pattern, --base or a --set-config-at pair. The checker checks them first (only those flags,
// see internal/web/link.go) and puts them before the page's own, so the page's controls win on the same config path.
let linkArgs = [];

// The flags that shape the result, only where they differ from the CLI's defaults: the checker, the Sphinx kit and
// the "Equivalent CLI flags" box all use these lists, so the box always shows what the page checked.
function cliArgs() {
  return [...linkArgs, ...pageArgs()];
}

// The flags from the page's controls.
function pageArgs() {
  const a = [];
  const interval = $("sample-interval").value;
  if (interval !== "1") a.push("--sample-interval", interval);
  const symbols = $("symbols").value;
  if (symbols !== "ascii") a.push("--ascii-symbols", symbols);
  const ds = $("downsample").value;
  if (ds === "1") a.push("--no-preview-downsample");
  else if (ds !== "15") a.push("--preview-downsample=" + ds);
  const states = statesOverride();
  if (states) a.push("--set-config-at", "states", "--set-config-value", states);
  const styles = paletteStyles();
  if (styles) a.push("--set-config-at", "representation.styles.states", "--set-config-value", styles);
  return a;
}

// pageArgs plus what only the page needs: the SVG follows the page's theme. The link's flags go separately.
function args() {
  return ["--svg-theme", state.theme === "system" ? "auto" : state.theme, ...pageArgs()];
}

function evaluate() {
  if (!ready) return;
  renderFlags();
  let res;
  try {
    res = JSON.parse(globalThis.cannedSchedules.evaluate(editor.value, args(), linkArgs));
  } catch (err) {
    showStatus("err", "Internal error: " + err);
    return;
  }
  render(res);
}

function render(res) {
  // State colours for the Sphinx admonitions come from the same config as the SVG.
  $("state-styles").textContent = res["sphinx-css"] || "";

  const valid = res.errors === 0;
  state.svg = res.svg || "";
  // The SVG comes from the tool's template, which escapes all text; the HTML previews come from goldmark in safe
  // mode (raw HTML dropped, dangerous links removed).
  $("svg").innerHTML = state.svg || '<p class="placeholder">Fix the errors to see the week.</p>';
  $("download-svg").disabled = !state.svg;
  $("download-kit").disabled = !state.svg;
  state.myst = res.myst || "";
  $("download-myst").disabled = !state.myst;
  state.text = res.text || "";
  $("download-text").disabled = !state.text;
  $("rendered").innerHTML = res[state.flavour + "-html"] || "";
  $("rendered").className = "rendered " + state.flavour;
  jsonView.value = res.json || "No output: the schedule has errors (see Diagnostics).";
  state.highlights = res.highlights || [];
  applyHighlight(editor.cursorLine); // After the JSON: replacing its text clears the viewer's marks.
  $("ascii").textContent = res.ascii || "No preview: the schedule has errors (see Diagnostics).";
  state.last = res;

  const report = JSON.parse(res.report);
  renderDiagnostics(report.diagnostics);
  $("states-panel").classList.toggle("invalid",
    report.diagnostics.some((d) => d.severity === "error" && d.code.startsWith("config-")));
  editor.setDiagnostics(report.diagnostics);

  const badge = $("badge");
  const count = res.errors + res.warnings;
  badge.hidden = count === 0;
  badge.textContent = String(count);
  badge.classList.toggle("warn", res.errors === 0);

  if (!valid) showStatus("err", plural(res.errors, "error") + (res.warnings ? ", " + plural(res.warnings, "warning") : ""));
  else if (res.warnings) showStatus("warn", "Valid, " + plural(res.warnings, "warning"));
  else showStatus("ok", "Valid: ready to paste into an issue");
}

function renderDiagnostics(list) {
  const ul = $("diagnostics");
  ul.replaceChildren();
  if (list.length === 0) {
    const li = document.createElement("li");
    li.className = "ok";
    li.append(text("span", "where", "✓"), text("span", "msg", "No problems found."));
    ul.append(li);
    return;
  }
  for (const d of list) {
    const li = document.createElement("li");
    li.className = d.severity === "warn" ? "warn" : "";
    if (d.line) {
      const b = text("button", "where", d.line + ":" + d.column);
      b.type = "button";
      b.title = "Show in the editor";
      b.addEventListener("click", () => editor.jumpTo(d.line, d.column));
      li.append(b);
    } else {
      li.append(text("span", "where", "–"));
    }
    li.append(text("span", "code-name", d.severity + " · " + d.code), text("span", "msg", d.message));
    if (d.example) li.append(exampleBlock(d.example));
    ul.append(li);
  }
}

// The example is the exact text to type, backticks included, so it's shown verbatim as code with a copy button.
function exampleBlock(example) {
  const wrap = document.createElement("div");
  wrap.className = "example";
  const pre = document.createElement("pre");
  pre.append(text("code", "", example));
  const copy = text("button", "ghost copy", "Copy");
  copy.type = "button";
  copy.title = "Copy the example";
  copy.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(example);
      copy.textContent = "Copied";
    } catch (_) {
      copy.textContent = "Copy failed";
    }
    setTimeout(() => { copy.textContent = "Copy"; }, 1500);
  });
  wrap.append(text("span", "example-label", "Example"), pre, copy);
  return wrap;
}

function text(tag, cls, content) {
  const el = document.createElement(tag);
  el.className = cls;
  el.textContent = content;
  return el;
}

function plural(n, word) { return n + " " + word + (n === 1 ? "" : "s"); }

function showStatus(kind, message) {
  const s = $("status");
  s.className = "status " + kind;
  s.replaceChildren(text("span", "dot", ""), document.createTextNode(message));
}

// ---- presets ------------------------------------------------------------------------------------------------------

async function loadPresets() {
  const list = await fetch(asset("presets/index.json")).then((r) => r.json());
  const texts = await Promise.all(list.map((p) => fetch(asset("presets/" + p.id + ".md")).then((r) => (r.ok ? r.text() : ""))));
  list.forEach((p, i) => presets.push({ ...p, text: texts[i] }));
  for (const p of presets) if (!p.hidden) addPresetOption(p);
}

// Hidden presets (an easter egg: ?preset=yolo) stay out of the dropdown until they are opened by link.
function addPresetOption(p) {
  $("preset").append(Object.assign(document.createElement("option"), { value: p.id, textContent: p.label }));
}

// Switching presets replaces the editor content; unsaved edits are only dropped after confirmation.
function choosePreset(id) {
  const next = presets.find((p) => p.id === id);
  if (!next) return;
  if (dirty() && !confirm("Replace your edits with the \"" + next.label + "\" preset?")) {
    $("preset").value = current.id;
    return;
  }
  current = next;
  load(next.text);
}

function load(text) {
  editor.value = text;
  updateReset();
  evaluate();
}

function dirty() {
  return current !== null && editor.value !== current.text;
}

// Reset is only active while the editor differs from the selected preset.
function updateReset() {
  $("reset").disabled = !dirty();
}

// ---- UI wiring -----------------------------------------------------------------------------------------------------

function selectTab(tab) {
  state.tab = tab;
  for (const b of document.querySelectorAll(".tabs button")) b.setAttribute("aria-selected", String(b.dataset.tab === tab));
  for (const p of document.querySelectorAll(".tab-panel")) p.hidden = p.dataset.panel !== tab;
  const json = document.querySelector('.tabs [data-tab="json"]');
  if (json) {
    // Opening the JSON tab answers the glow; leaving it with marks shows the steady glow again, without a pulse.
    json.classList.toggle("has-marks", tab !== "json" && lastMarks !== "");
    json.classList.remove("pulse");
    json.title = tab !== "json" && lastMarks ? "Lines for the cursor's entry are marked in the JSON" : "";
  }
}

function debounce(fn, ms) {
  let t;
  return () => { clearTimeout(t); t = setTimeout(fn, ms); };
}

function download(name, type, content) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name });
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// The kit is built on demand: stylesheet, fonts, a local Sphinx extension and the current schedule as MyST.
function downloadKit() {
  const zip = globalThis.cannedSchedules.sphinxKit(editor.value, args(), linkArgs);
  if (typeof zip === "string") {
    showStatus("err", zip);
    return;
  }
  download("canned-schedules-sphinx-kit.zip", "application/zip", zip);
}

async function copyMarkdown() {
  const b = $("copy-md");
  try {
    await navigator.clipboard.writeText(editor.value);
    b.textContent = "Copied";
  } catch (_) {
    editor.selectAll();
    b.textContent = "Select all + copy";
  }
  setTimeout(() => { b.textContent = "Copy Markdown"; }, 1500);
}

// ---- resizable editor ----------------------------------------------------------------------------------------------

// The splitter between the panes sets the editor's share of the layout width (--editor-width, in percent): drag it,
// use the arrow keys (Home/End for the limits), or double-click it to reset. Remembered per browser.
const SPLIT = { min: 20, max: 70, initial: 33.3, step: 2, key: "editor-width" };
let split = SPLIT.initial;

function setSplit(pct, save) {
  split = Math.min(SPLIT.max, Math.max(SPLIT.min, pct));
  $("splitter").parentElement.style.setProperty("--editor-width", split + "%");
  $("splitter").setAttribute("aria-valuenow", String(Math.round(split)));
  if (save) try { localStorage.setItem(SPLIT.key, String(split)); } catch (_) {}
}

function wireSplitter() {
  const handle = $("splitter");
  const layout = handle.parentElement;
  let saved = NaN;
  try { saved = parseFloat(localStorage.getItem(SPLIT.key)); } catch (_) {}
  setSplit(Number.isFinite(saved) ? saved : SPLIT.initial, false);

  handle.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    try { handle.setPointerCapture(e.pointerId); } catch (_) {} // Keeps the drag when the pointer leaves the handle.
    layout.classList.add("resizing");
    const move = (ev) => {
      const r = layout.getBoundingClientRect();
      setSplit(((ev.clientX - r.left) / r.width) * 100, false);
    };
    const end = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", end);
      handle.removeEventListener("pointercancel", end);
      layout.classList.remove("resizing");
      setSplit(split, true);
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", end);
    handle.addEventListener("pointercancel", end);
  });
  handle.addEventListener("keydown", (e) => {
    const next = { ArrowLeft: split - SPLIT.step, ArrowRight: split + SPLIT.step, Home: SPLIT.min, End: SPLIT.max }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setSplit(next, true);
  });
  handle.addEventListener("dblclick", () => setSplit(SPLIT.initial, true));
}

// ---- states ------------------------------------------------------------------------------------------------------

// The states editor: rows of {id, name} sent as --set-config-at states, only while they differ from the built-in
// states. An empty name means the id. The checker validates the rows like the CLI does (config-* diagnostics).
let defaultStates = []; // [{id, name}], from the embedded config
let stateRows = [];
let queryStates = null; // ?states=… entries, applied once the defaults are known

// Asks the checker for the built-in states: an empty document still loads the config, so the result names them, and
// the defaults stay in schema/config.json only. A link's flags count: states it sets are the defaults then.
function loadDefaultStates() {
  const res = JSON.parse(globalThis.cannedSchedules.evaluate("", [], linkArgs));
  defaultStates = (res.states || []).map((s) => ({ id: s.id, name: s["display-name"] }));
}

const displayName = (row) => row.name || row.id;

function sameStates(a, b) {
  return a.length === b.length && a.every((r, i) => r.id === b[i].id && displayName(r) === displayName(b[i]));
}

function statesOverride() {
  if (sameStates(stateRows, defaultStates)) return null;
  return JSON.stringify(stateRows.map((r) => ({ id: r.id, "display-name": displayName(r) })));
}

// ?states=Primary&states=Secondary:Backup%20on-call: the id, or the id and the display name split at the first colon
// (ids can't contain one). Entries that don't validate are passed on; the checker reports them.
function parseStates(entries) {
  return entries.map((e) => {
    const at = e.indexOf(":");
    return at < 0 ? { id: e, name: "" } : { id: e.slice(0, at), name: e.slice(at + 1) };
  });
}

function setStates(rows) {
  stateRows = rows.map((r) => ({ ...r }));
  renderStates();
}

// Rebuilt only when rows are added, removed or moved; typing updates the row in place, so the focus stays.
function renderStates() {
  const ol = $("state-rows");
  ol.replaceChildren();
  stateRows.forEach((row, i) => {
    const li = document.createElement("li");
    const div = Object.assign(document.createElement("div"), { className: "state-row" });
    const id = Object.assign(document.createElement("input"), {
      className: "state-id", value: row.id, spellcheck: false, autocomplete: "off",
    });
    id.setAttribute("aria-label", "State " + (i + 1) + " id");
    const name = Object.assign(document.createElement("input"), {
      className: "state-name", value: row.name, placeholder: row.id, autocomplete: "off",
    });
    name.setAttribute("aria-label", "State " + (i + 1) + " display name");
    id.addEventListener("input", () => { row.id = id.value; name.placeholder = id.value; statesChanged(); });
    name.addEventListener("input", () => { row.name = name.value; statesChanged(); });
    div.append(id, name,
      rowButton("↑", "Move state " + (i + 1) + " up", i === 0, () => moveState(i, -1)),
      rowButton("↓", "Move state " + (i + 1) + " down", i === stateRows.length - 1, () => moveState(i, 1)),
      rowButton("✕", "Remove state " + (i + 1), stateRows.length === 1, () => { stateRows.splice(i, 1); structureChanged(); }));
    li.append(div);
    ol.append(li);
  });
  updateStatesSummary();
}

function rowButton(label, title, disabled, onClick) {
  const b = text("button", "ghost", label);
  Object.assign(b, { type: "button", title, disabled });
  b.setAttribute("aria-label", title);
  b.addEventListener("click", onClick);
  return b;
}

function moveState(i, by) {
  [stateRows[i], stateRows[i + by]] = [stateRows[i + by], stateRows[i]];
  structureChanged();
}

function addState() {
  const taken = new Set(stateRows.map((r) => r.id.toUpperCase()));
  let n = stateRows.length + 1;
  while (taken.has("STATE" + n)) n++;
  stateRows.push({ id: "State" + n, name: "" });
  structureChanged();
  const inputs = $("state-rows").querySelectorAll("input.state-id");
  inputs[inputs.length - 1].select();
}

function updateStatesSummary() {
  const custom = !sameStates(stateRows, defaultStates);
  $("states-summary").textContent = "· " + stateRows.length + (custom ? ", custom" : ", built-in");
  $("reset-states").disabled = !custom;
}

const evaluateSoon = debounce(evaluate, 120);

function statesChanged() {
  updateStatesSummary();
  evaluateSoon();
}

function structureChanged() {
  renderStates();
  evaluate();
}

// ---- palettes ----------------------------------------------------------------------------------------------------

// Colour palettes (palettes/index.json; TestPalettes keeps them valid) replace representation.styles.states, the same
// override the CLI takes. The first, "built-in", has no styles: the config's own. Sequential palettes suit ordered
// states, categorical ones unordered states such as rotations.
const palettes = [];
let queryPalette = null; // ?palette=…, applied once the palettes are loaded

async function loadPalettes() {
  palettes.push(...(await fetch(asset("palettes/index.json")).then((r) => r.json())));
  const groups = { sequential: "Sequential (ordered states)", categorical: "Categorical (unordered states)" };
  for (const [kind, label] of Object.entries(groups)) {
    const group = Object.assign(document.createElement("optgroup"), { label });
    for (const p of palettes.filter((x) => x.kind === kind)) {
      group.append(Object.assign(document.createElement("option"), { value: p.id, textContent: p.label, title: p.description }));
    }
    $("palette").append(group);
  }
  if (queryPalette && palettes.some((p) => p.id === queryPalette)) $("palette").value = queryPalette;
  else $("palette").value = palettes[0].id;
  describePalette();
}

function paletteStyles() {
  const p = palettes.find((x) => x.id === $("palette").value);
  return p && p.styles ? JSON.stringify(p.styles) : null;
}

function describePalette() {
  const p = palettes.find((x) => x.id === $("palette").value);
  $("palette").title = p ? p.description : "";
}

// ---- CLI flags ---------------------------------------------------------------------------------------------------

let flagsFormat = "shell";

// Quotes an argument for POSIX shells when it needs it: single quotes, with ' written as '\''.
function shellQuote(arg) {
  return /^[A-Za-z0-9_.\/=:,@%+-]+$/.test(arg) ? arg : "'" + arg.replace(/'/g, "'\\''") + "'";
}

// One line per flag with its value: as a shell command, or one argument per line as canned-schedules.args reads them.
function flagsText(list) {
  if (flagsFormat === "args") return list.join("\n") + "\n";
  const lines = [];
  for (const arg of list) {
    if (arg.startsWith("--") || lines.length === 0) lines.push(shellQuote(arg));
    else lines[lines.length - 1] += " " + shellQuote(arg);
  }
  return ["canned-schedules", ...lines, "schedule.md"].join(" \\\n  ") + "\n";
}

function renderFlags() {
  const list = cliArgs();
  const none = list.length === 0;
  $("flags").textContent = none ? "Defaults only: no flags needed." : flagsText(list);
  $("flags").classList.toggle("muted", none);
  $("copy-flags").disabled = none;
  const n = list.filter((a) => a.startsWith("--")).length;
  $("flags-summary").textContent = "· " + (none ? "none" : plural(n, "flag"));
}

// The note under the options while the link sets flags of its own; they show in the "Equivalent CLI flags" box.
function renderLinkArgs() {
  const n = linkArgs.filter((a) => a.startsWith("-")).length;
  $("link-args").hidden = linkArgs.length === 0;
  $("link-args-count").textContent = plural(n, "flag");
}

// Drops the link's flags. The states panel follows the defaults when it showed them.
function dropLinkArgs() {
  const followed = !statesOverride();
  linkArgs = [];
  loadDefaultStates();
  if (followed) setStates(defaultStates);
  renderLinkArgs();
  evaluate();
}

async function copyFlags() {
  const b = $("copy-flags");
  try {
    await navigator.clipboard.writeText($("flags").textContent);
    b.textContent = "Copied";
  } catch (_) {
    b.textContent = "Copy failed";
  }
  setTimeout(() => { b.textContent = "Copy"; }, 1500);
}

// ---- permalink -----------------------------------------------------------------------------------------------------

// The page state as a URL: the preset, the form controls and the views as query parameters, and the document in the
// fragment as base64url, only when it differs from the preset. Fragments never reach the server.
const CONTROLS = ["downsample", "sample-interval", "symbols"];

function permalink() {
  const q = new URLSearchParams();
  if (current) q.set("preset", current.id);
  if (state.theme !== "system") q.set("theme", state.theme);
  q.set("view", state.flavour);
  q.set("tab", state.tab);
  for (const id of CONTROLS) q.set(id, $(id).value);
  if ($("palette").value !== palettes[0]?.id) q.set("palette", $("palette").value);
  if (statesOverride()) for (const r of stateRows) q.append("states", r.name && r.name !== r.id ? r.id + ":" + r.name : r.id);
  for (const a of linkArgs) q.append("arg", a);
  const url = new URL(location.pathname, location.origin);
  url.search = q.toString();
  if (dirty()) url.hash = "doc=" + toBase64url(editor.value);
  return url.href;
}

function toBase64url(text) {
  const bytes = new TextEncoder().encode(text);
  let bin = "";
  // In chunks: spreading a large array into fromCharCode overflows the call stack.
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// Copies the permalink and puts it in the address bar, so a reload keeps the state as well.
async function copyLink() {
  const url = permalink();
  history.replaceState(null, "", url);
  const b = $("copy-link");
  try {
    await navigator.clipboard.writeText(url);
    b.textContent = "Link copied";
  } catch (_) {
    b.textContent = "Link in address bar";
  }
  setTimeout(() => { b.textContent = "Copy link"; }, 1500);
}

function wire() {
  wireSplitter();
  for (const b of $("theme").querySelectorAll("button")) b.addEventListener("click", () => setTheme(b.dataset.value));
  for (const b of $("flavour").querySelectorAll("button")) {
    b.addEventListener("click", () => {
      state.flavour = b.dataset.value;
      pressed($("flavour"), state.flavour);
      if (state.last) render(state.last);
    });
  }
  for (const b of document.querySelectorAll(".tabs button")) b.addEventListener("click", () => selectTab(b.dataset.tab));
  for (const id of ["downsample", "sample-interval", "symbols"]) $(id).addEventListener("change", evaluate);
  $("palette").addEventListener("change", () => { describePalette(); evaluate(); });
  $("add-state").addEventListener("click", addState);
  $("reset-states").addEventListener("click", () => { setStates(defaultStates); evaluate(); });
  for (const b of $("flags-format").querySelectorAll("button")) {
    b.addEventListener("click", () => {
      flagsFormat = b.dataset.value;
      pressed($("flags-format"), flagsFormat);
      renderFlags();
    });
  }
  $("copy-flags").addEventListener("click", copyFlags);
  $("drop-link-args").addEventListener("click", dropLinkArgs);
  editor.onChange(debounce(evaluate, 120));
  // Loading a document moves the cursor too; only a cursor in the focused editor is the user's.
  editor.onCursor((line) => { cursorUsed ||= editor.hasFocus; applyHighlight(line); });
  editor.onChange(updateReset);
  $("preset").addEventListener("change", () => choosePreset($("preset").value));
  $("reset").addEventListener("click", () => { if (current) load(current.text); });
  $("copy-md").addEventListener("click", copyMarkdown);
  $("copy-link").addEventListener("click", copyLink);
  $("download-svg").addEventListener("click", () => download("schedule.svg", "image/svg+xml", state.svg));
  $("download-myst").addEventListener("click", () => download("schedule.md", "text/markdown", state.myst));
  $("download-text").addEventListener("click", () => download("schedule.txt", "text/plain;charset=utf-8", state.text));
  $("download-kit").addEventListener("click", downloadKit);
  // Re-render when the OS theme changes while following the system (the SVG uses --svg-theme auto then).
  matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => { if (state.theme === "system") evaluate(); });
  pressed($("theme"), state.theme);
}

// ---- start ---------------------------------------------------------------------------------------------------------

// A document can be passed in the URL fragment: #doc=<base64url of UTF-8 Markdown>. Fragments never reach the server.
function fromHash() {
  const m = /(?:^#|&)doc=([A-Za-z0-9_-]+)/.exec(location.hash);
  if (!m) return null;
  try {
    const b64 = m[1].replace(/-/g, "+").replace(/_/g, "/");
    const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch (_) {
    return null;
  }
}

// Views can be linked: ?preset=minimal&theme=dark&view=sphinx&tab=json&downsample=30&sample-interval=15&symbols=braille
// &palette=okabe-ito&states=Primary&states=Off&arg=--base&arg=Off (see permalink). Unknown values of the controls are
// ignored; ?arg= goes to the checker as it is, which reports what it can't take.
function applyQuery() {
  const q = new URLSearchParams(location.search);
  const theme = q.get("theme");
  if (theme === "light" || theme === "dark" || theme === "system") {
    state.theme = theme;
    document.documentElement.dataset.theme = theme;
  }
  pressed($("theme"), state.theme);
  const view = q.get("view");
  if (view === "github" || view === "sphinx") state.flavour = view;
  pressed($("flavour"), state.flavour);
  const preset = q.get("preset");
  if (preset) state.preset = preset;
  for (const id of CONTROLS) {
    const v = q.get(id);
    if (v !== null && Array.from($(id).options).some((o) => o.value === v)) $(id).value = v;
  }
  queryPalette = q.get("palette");
  if (q.has("states")) queryStates = parseStates(q.getAll("states"));
  linkArgs = q.getAll("arg");
  const tab = q.get("tab");
  if (["rendered", "diagnostics", "json", "ascii"].includes(tab)) selectTab(tab);
}

// fetchWasm downloads app.wasm and shows the progress in the loading overlay. The total is only shown when the
// server sends the file uncompressed: with Content-Encoding, Content-Length counts compressed bytes while the
// stream delivers decompressed ones.
async function fetchWasm() {
  const res = await fetch(asset("app.wasm"));
  if (!res.ok) throw new Error("app.wasm: HTTP " + res.status);
  const encoded = (res.headers.get("Content-Encoding") || "identity") !== "identity";
  const total = encoded ? 0 : Number(res.headers.get("Content-Length")) || 0;
  const mb = (n) => (n / 1e6).toFixed(1) + " MB";
  const chunks = [];
  let loaded = 0;
  const reader = res.body.getReader();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    loaded += value.length;
    $("hud-progress").textContent = total
      ? mb(loaded) + " of " + mb(total) + " (" + Math.floor((loaded / total) * 100) + " %)"
      : mb(loaded) + " loaded";
  }
  const bytes = new Uint8Array(loaded);
  let at = 0;
  for (const c of chunks) {
    bytes.set(c, at);
    at += c.length;
  }
  $("hud-progress").textContent = "Starting…";
  return bytes;
}

// The loading overlay stays until the checker runs; on failure it shows why, as the status line does.
function hudError(message) {
  $("hud").classList.add("error");
  $("hud-text").textContent = message;
  showStatus("err", message);
}

async function start() {
  // The logo leads to the page itself, without settings or document.
  $("home").href = location.pathname;
  wire();
  applyQuery();
  if (!("WebAssembly" in globalThis)) {
    hudError("This browser can't run WebAssembly.");
    return;
  }
  try {
    const go = new Go();
    // Downloaded by hand (not instantiateStreaming) to show the progress; compiling afterwards costs little next to
    // the download.
    const [instance] = await Promise.all([
      fetchWasm().then((b) => WebAssembly.instantiate(b, go.importObject)).then((r) => r.instance),
      loadPresets(),
      loadPalettes(),
    ]);
    go.run(instance);
    loadDefaultStates();
    setStates(queryStates || defaultStates);
    renderLinkArgs();
    // A link with custom states opens the editor, so the reader sees why the schedule checks against them.
    if (queryStates) $("states-panel").open = true;
    current = presets.find((p) => p.id === state.preset) || presets.find((p) => p.id === DEFAULT_PRESET) || presets[0];
    if (current.hidden) addPresetOption(current);
    $("preset").value = current.id;
    editor.value = fromHash() ?? current.text;
    updateReset();
    ready = true;
    $("hud").hidden = true;
    evaluate();
  } catch (err) {
    hudError("Couldn't load the checker: " + err);
  }
}

start();
