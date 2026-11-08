// Cache busting for the built page (web:build runs this last): every file the page loads gets a content hash in its
// URL, so a browser never pairs files of two deploys. GitHub Pages lets browsers reuse each file for 10 minutes on
// its own clock; without hashes a new app.js could meet an old app.wasm (their evaluate result has no version).
//
// - index.html: the scripts and the stylesheet it loads get `?v=<hash>`.
// - index.html also gets a JSON manifest (#asset-versions) with the hashes of the files app.js fetches (app.wasm,
//   presets, palettes); app.js appends them itself (`asset()`). Without the manifest (web/ served raw) it uses
//   plain names.
// - index.html stays unhashed: after a deploy it may be up to 10 minutes old, but everything it pulls in matches it.
// - Fonts and schemas are left alone: fonts practically never change, schemas live under dated paths.
//
// Hashes are per file, so an unchanged file (the 400 KB editor bundle, the 10 MB checker) stays cached across
// deploys. Usage: node editor/stamp-assets.mjs <out-dir>
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

const out = process.argv[2];
if (!out) throw new Error("usage: node editor/stamp-assets.mjs <out-dir>");

const hash = (rel) => createHash("sha256").update(fs.readFileSync(path.join(out, rel))).digest("hex").slice(0, 12);

// Loaded by index.html.
const LOADED = ["style.css", "editor.js", "wasm_exec.js", "app.js"];
// Fetched by app.js.
const listed = (dir) => fs.readdirSync(path.join(out, dir)).sort().map((f) => `${dir}/${f}`);
const fetched = ["app.wasm", ...listed("presets"), ...listed("palettes")];

const indexPath = path.join(out, "index.html");
let html = fs.readFileSync(indexPath, "utf8");

for (const rel of LOADED) {
  const re = new RegExp(`((?:src|href)=")${rel.replace(".", "\\.")}(")`, "g");
  const n = (html.match(re) || []).length;
  if (n !== 1) throw new Error(`index.html: expected one reference to ${rel}, found ${n}`);
  html = html.replace(re, `$1${rel}?v=${hash(rel)}$2`);
}

const manifest = Object.fromEntries(fetched.map((rel) => [rel, hash(rel)]));
const anchor = '<script src="editor.js?v=';
if (!html.includes(anchor)) throw new Error("index.html: no editor.js script to put the manifest before");
html = html.replace(anchor, `<script type="application/json" id="asset-versions">${JSON.stringify(manifest)}</script>\n${anchor}`);

// Every local script or stylesheet must carry a hash now; a new one added to index.html without updating LOADED fails.
for (const m of html.matchAll(/<(?:script[^>]*\ssrc|link[^>]*rel="stylesheet"[^>]*\shref)="([^"]+)"/g)) {
  if (!/^[a-z]+:|^\/\//.test(m[1]) && !m[1].includes("?v=")) throw new Error(`index.html: ${m[1]} has no content hash`);
}

fs.writeFileSync(indexPath, html);
console.log(`stamped ${LOADED.length + fetched.length} assets`);
