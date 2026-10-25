// The preflight page's editor (Markdown) and JSON viewer, built on CodeMirror 6. Bundled into editor.js, which sets
// globalThis.CannedEditor; app.js falls back to the plain textarea when it is missing.
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
// Not markdown(): it keeps the HTML, CSS and JavaScript modes as defaults (for embedded HTML), tripling the bundle.
import { deleteMarkupBackward, insertNewlineContinueMarkup, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, LanguageSupport, foldGutter, syntaxHighlighting } from "@codemirror/language";
import { Prec } from "@codemirror/state";
import { lintGutter, setDiagnostics } from "@codemirror/lint";
import { highlightSelectionMatches, search, searchKeymap } from "@codemirror/search";
import { EditorSelection, EditorState, RangeSetBuilder, StateEffect, StateField } from "@codemirror/state";
import {
  Decoration, EditorView, MatchDecorator, ViewPlugin, drawSelection, highlightActiveLine, highlightActiveLineGutter,
  keymap, lineNumbers,
} from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

// Colours are CSS variables (web/style.css), so the light/dark toggle needs no reconfiguration.
const highlight = HighlightStyle.define([
  { tag: t.heading, color: "var(--hl-heading)", fontWeight: "700" },
  { tag: [t.processingInstruction, t.contentSeparator], color: "var(--hl-mark)" },
  { tag: t.comment, color: "var(--hl-quote)", fontStyle: "italic" },
  { tag: t.monospace, color: "var(--hl-code)" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strong, fontWeight: "700" },
  { tag: [t.link, t.url], color: "var(--hl-link)" },
  { tag: t.propertyName, color: "var(--hl-key)" },
  { tag: t.string, color: "var(--hl-string)" },
  { tag: t.number, color: "var(--hl-number)" },
  { tag: [t.bool, t.null], color: "var(--hl-keyword)" },
  { tag: [t.brace, t.squareBracket, t.separator, t.punctuation], color: "var(--hl-punct)" },
]);

const theme = EditorView.theme({
  "&": { color: "var(--text)", backgroundColor: "var(--surface-2)", fontSize: "15px" },
  ".cm-content": { fontFamily: "var(--mono)", lineHeight: "1.45", caretColor: "var(--text)" },
  ".cm-gutters": { backgroundColor: "var(--surface-2)", color: "var(--muted)", border: "none" },
  ".cm-activeLine": { backgroundColor: "var(--hl-active-line)" },
  ".cm-activeLineGutter": { backgroundColor: "var(--hl-active-line)" },
  "&.cm-focused .cm-cursor": { borderLeftColor: "var(--text)" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": {
    backgroundColor: "var(--hl-selection) !important",
  },
});

// Schedule markers ([!SCHEDULE:STATE], [!NOTE]) are plain text to Markdown; give them their own colour.
const markers = new MatchDecorator({
  regexp: /^(?:\s*>\s*)\[![A-Za-z][A-Za-z0-9:-]*\]/g,
  decorate: (add, from, to, match) => {
    const start = from + match[0].indexOf("[!");
    add(start, to, Decoration.mark({ class: match[0].includes(":") ? "cm-schedule-marker" : "cm-alert-marker" }));
  },
});
const markerPlugin = ViewPlugin.define((view) => ({
  decorations: markers.createDeco(view),
  update(u) { this.decorations = markers.updateDeco(u, this.decorations); },
}), { decorations: (p) => p.decorations });

// Lines with diagnostics get a background: errors win over warnings on the same line.
const setLines = StateEffect.define();
const diagnosticLines = StateField.define({
  create: () => Decoration.none,
  update(deco, tr) {
    deco = deco.map(tr.changes);
    for (const e of tr.effects) if (e.is(setLines)) deco = e.value;
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

// offset converts the tool's 1-based line and column (columns count code points) into a document offset.
function offset(doc, line, column) {
  const l = doc.line(Math.min(Math.max(line, 1), doc.lines));
  let pos = l.from;
  let col = 1;
  for (const ch of l.text) {
    if (col >= column) break;
    pos += ch.length;
    col++;
  }
  return { pos, line: l };
}

function toDiagnostics(doc, list) {
  const diags = [];
  const lines = new Map(); // line number → "error" | "warn"
  for (const d of list) {
    if (!d.line) continue;
    const { pos, line } = offset(doc, d.line, d.column || 1);
    // Underline the word at the position (up to the next space), at least one character where there is one.
    const rest = /^\S*/.exec(doc.sliceString(pos, line.to))[0];
    const to = Math.min(line.to, pos + Math.max(rest.length, 1));
    const severity = d.severity === "warn" ? "warning" : "error";
    diags.push({ from: pos, to, severity, source: d.code, message: d.message });
    if (lines.get(line.number) !== "error") lines.set(line.number, d.severity === "warn" ? "warn" : "error");
  }
  const builder = new RangeSetBuilder();
  for (const n of [...lines.keys()].sort((a, b) => a - b)) {
    builder.add(doc.line(n).from, doc.line(n).from, Decoration.line({ class: "cm-diag-line-" + lines.get(n) }));
  }
  return { diags, lines: builder.finish() };
}

function createEditor(parent, { doc = "", label = "", onChange = () => {} } = {}) {
  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc,
      extensions: [
        lineNumbers(), highlightActiveLineGutter(), lintGutter(), history(), drawSelection(), highlightActiveLine(),
        search(), highlightSelectionMatches(), EditorView.lineWrapping, EditorState.tabSize.of(2),
        keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
        new LanguageSupport(markdownLanguage), syntaxHighlighting(highlight),
        // Enter continues lists and quotes ("> - "), Backspace removes their markup, as markdown() would set up.
        Prec.high(keymap.of([
          { key: "Enter", run: insertNewlineContinueMarkup },
          { key: "Backspace", run: deleteMarkupBackward },
        ])), markerPlugin, diagnosticLines, theme,
        EditorView.contentAttributes.of({ "aria-label": label, spellcheck: "false", autocapitalize: "off" }),
        EditorView.updateListener.of((u) => { if (u.docChanged) onChange(); }),
      ],
    }),
  });

  return {
    view,
    get value() { return view.state.doc.toString(); },
    set value(text) { view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } }); },
    focus() { view.focus(); },
    selectAll() { view.dispatch({ selection: EditorSelection.single(0, view.state.doc.length) }); view.focus(); },
    jumpTo(line, column) {
      const { pos, line: l } = offset(view.state.doc, line, column);
      view.dispatch({
        selection: EditorSelection.single(pos, Math.min(l.to, pos + 1)),
        effects: EditorView.scrollIntoView(pos, { y: "center" }),
      });
      view.focus();
    },
    setDiagnostics(list) {
      const { diags, lines } = toDiagnostics(view.state.doc, list);
      view.dispatch(setDiagnostics(view.state, diags));
      view.dispatch({ effects: setLines.of(lines) });
    },
  };
}

// createViewer shows read-only JSON with highlighting and folding.
function createViewer(parent, { label = "" } = {}) {
  const view = new EditorView({
    parent,
    state: EditorState.create({
      extensions: [
        lineNumbers(), foldGutter(), EditorState.readOnly.of(true), EditorView.editable.of(false), json(),
        syntaxHighlighting(highlight), search(), keymap.of(searchKeymap), theme,
        EditorView.contentAttributes.of({ "aria-label": label }),
      ],
    }),
  });

  return {
    view,
    set value(text) { view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } }); },
  };
}

globalThis.CannedEditor = { createEditor, createViewer };
