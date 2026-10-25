// canned-schedules preflight: runs the Go pipeline (compiled to WebAssembly) on every edit. Nothing leaves the page.
"use strict";

const $ = (id) => document.getElementById(id);
const editor = createEditor($("editor"));
const jsonView = createJsonView($("json"));
const DEFAULT_PRESET = "office-hours";
const state = { theme: document.documentElement.dataset.theme || "system", flavour: "sphinx", tab: "rendered", svg: "", myst: "", preset: "" };
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

function args() {
  const svgTheme = state.theme === "system" ? "auto" : state.theme;
  const a = ["--svg-theme", svgTheme, "--sample-interval", $("sample-interval").value, "--ascii-symbols", $("symbols").value];
  const ds = $("downsample").value;
  a.push(ds === "1" ? "--no-preview-downsample" : "--preview-downsample=" + ds);
  return a;
}

function evaluate() {
  if (!ready) return;
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
  $("rendered").innerHTML = res[state.flavour + "-html"] || "";
  $("rendered").className = "rendered " + state.flavour;
  jsonView.value = res.json || "No output: the request has errors (see Diagnostics).";
  $("ascii").textContent = res.ascii || "No preview: the request has errors (see Diagnostics).";
  state.last = res;

  const report = JSON.parse(res.report);
  renderDiagnostics(report.diagnostics);
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
  const select = $("preset");
  for (const p of presets) select.append(Object.assign(document.createElement("option"), { value: p.id, textContent: p.label }));
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

function wire() {
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
  editor.onChange(debounce(evaluate, 120));
  editor.onChange(updateReset);
  $("preset").addEventListener("change", () => choosePreset($("preset").value));
  $("reset").addEventListener("click", () => { if (current) load(current.text); });
  $("copy-md").addEventListener("click", copyMarkdown);
  $("download-svg").addEventListener("click", () => download("schedule.svg", "image/svg+xml", state.svg));
  $("download-myst").addEventListener("click", () => download("schedule.md", "text/markdown", state.myst));
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

// Views can be linked: ?theme=dark&view=sphinx&tab=json&symbols=braille&preset=minimal
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
  const symbols = q.get("symbols");
  if (symbols === "ascii" || symbols === "braille") $("symbols").value = symbols;
  const tab = q.get("tab");
  if (["rendered", "diagnostics", "json", "ascii"].includes(tab)) selectTab(tab);
}

async function start() {
  wire();
  applyQuery();
  if (!("WebAssembly" in globalThis)) {
    showStatus("err", "This browser can't run WebAssembly.");
    return;
  }
  try {
    const go = new Go();
    const wasm = fetch("app.wasm");
    const [instance] = await Promise.all([
      WebAssembly.instantiateStreaming
        ? WebAssembly.instantiateStreaming(wasm, go.importObject).then((r) => r.instance)
        : wasm.then((r) => r.arrayBuffer()).then((b) => WebAssembly.instantiate(b, go.importObject)).then((r) => r.instance),
      loadPresets(),
    ]);
    go.run(instance);
    current = presets.find((p) => p.id === state.preset) || presets.find((p) => p.id === DEFAULT_PRESET) || presets[0];
    $("preset").value = current.id;
    editor.value = fromHash() ?? current.text;
    updateReset();
    ready = true;
    const v = await fetch("version.txt").then((r) => (r.ok ? r.text() : "")).catch(() => "");
    $("version").textContent = v.trim();
    evaluate();
  } catch (err) {
    showStatus("err", "Couldn't load the checker: " + err);
  }
}

start();
