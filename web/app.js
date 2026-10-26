// canned-schedules preflight: runs the Go pipeline (compiled to WebAssembly) on every edit. Nothing leaves the page.
"use strict";

const $ = (id) => document.getElementById(id);
const editor = createEditor($("editor"));
const jsonView = createJsonView($("json"));
const DEFAULT_PRESET = "office-hours";
const state = { theme: document.documentElement.dataset.theme || "system", flavour: "sphinx", tab: "rendered", svg: "", myst: "", text: "", preset: "" };
const presets = []; // [{id, label, text}], from presets/index.json
let current = null; // the selected preset
let ready = false;

// ---- editor ------------------------------------------------------------------------------------------------------

// The editor is CodeMirror (editor.js) with Markdown highlighting and diagnostics marked in place. Without editor.js
// the plain textarea stays, behind the same small interface: value, onChange, focus, selectAll, jumpTo,
// setDiagnostics.
function createEditor(textarea) {
  const listeners = [];
  const changed = () => { for (const fn of listeners) fn(); };

  if (globalThis.CannedEditor) {
    const host = Object.assign(document.createElement("div"), { className: "cm-host editor" });
    textarea.replaceWith(host);
    const cm = globalThis.CannedEditor.createEditor(host, { label: textarea.getAttribute("aria-label"), onChange: changed });
    cm.onChange = (fn) => listeners.push(fn);
    return cm;
  }

  textarea.addEventListener("input", changed);
  return {
    get value() { return textarea.value; },
    set value(text) { textarea.value = text; },
    onChange: (fn) => listeners.push(fn),
    focus: () => textarea.focus(),
    selectAll: () => textarea.select(),
    setDiagnostics() {},
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
  if (!globalThis.CannedEditor) return { set value(text) { pre.textContent = text; } };
  const host = Object.assign(document.createElement("div"), { className: "cm-host viewer", id: "json" });
  pre.replaceWith(host);
  return globalThis.CannedEditor.createViewer(host, { label: "Canonical JSON output" });
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

// The flags that shape the result, only where they differ from the CLI's defaults: the checker, the Sphinx kit and
// the "Equivalent CLI flags" box all use this one list, so the box always shows what the page checked.
function cliArgs() {
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

// cliArgs plus what only the page needs: the SVG follows the page's theme.
function args() {
  return ["--svg-theme", state.theme === "system" ? "auto" : state.theme, ...cliArgs()];
}

function evaluate() {
  if (!ready) return;
  renderFlags();
  let res;
  try {
    res = JSON.parse(globalThis.cannedSchedules.evaluate(editor.value, args()));
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
  jsonView.value = res.json || "No output: the request has errors (see Diagnostics).";
  $("ascii").textContent = res.ascii || "No preview: the request has errors (see Diagnostics).";
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
  const list = await fetch("presets/index.json").then((r) => r.json());
  const texts = await Promise.all(list.map((p) => fetch("presets/" + p.id + ".md").then((r) => (r.ok ? r.text() : ""))));
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

// The kit is built on demand: stylesheet, fonts, a local Sphinx extension and the current request as MyST.
function downloadKit() {
  const zip = globalThis.cannedSchedules.sphinxKit(editor.value, args());
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
// the defaults stay in schema/config.json only.
function loadDefaultStates() {
  const res = JSON.parse(globalThis.cannedSchedules.evaluate("", []));
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
  palettes.push(...(await fetch("palettes/index.json").then((r) => r.json())));
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
  return ["canned-schedules", ...lines, "request.md"].join(" \\\n  ") + "\n";
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
  editor.onChange(debounce(evaluate, 120));
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
// &palette=okabe-ito&states=Primary&states=Off (see permalink). Unknown values are ignored.
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
  const tab = q.get("tab");
  if (["rendered", "diagnostics", "json", "ascii"].includes(tab)) selectTab(tab);
}

// fetchWasm downloads app.wasm and shows the progress in the loading overlay. The total is only shown when the
// server sends the file uncompressed: with Content-Encoding, Content-Length counts compressed bytes while the
// stream delivers decompressed ones.
async function fetchWasm() {
  const res = await fetch("app.wasm");
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
    // A link with custom states opens the editor, so the reader sees why the request checks against them.
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
