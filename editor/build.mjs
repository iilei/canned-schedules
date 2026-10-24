// Bundles src/editor.js into <out>/editor.js and writes the licences of every bundled package to
// <out>/editor-licenses.txt. Usage: node editor/build.mjs dist/site
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import * as esbuild from "esbuild";

const here = path.dirname(fileURLToPath(import.meta.url));
const out = path.resolve(process.argv[2] ?? "dist/site");

const result = await esbuild.build({
  absWorkingDir: here,
  entryPoints: ["src/editor.js"],
  bundle: true,
  minify: true,
  format: "iife",
  target: "es2020",
  legalComments: "none",
  banner: { js: "/* canned-schedules editor: CodeMirror 6 and friends (MIT); licences in editor-licenses.txt */" },
  outfile: path.join(out, "editor.js"),
  metafile: true,
});

// Every package with code in the bundle. The metafile's inputs also list files that tree shaking dropped entirely.
const packages = new Set();
for (const output of Object.values(result.metafile.outputs)) {
  for (const [input, { bytesInOutput }] of Object.entries(output.inputs)) {
    const m = /node_modules\/((?:@[^/]+\/)?[^/]+)\//.exec(input);
    if (m && bytesInOutput > 0) packages.add(m[1]);
  }
}

const sections = [...packages].sort().map((name) => {
  const dir = path.join(here, "node_modules", name);
  const pkg = JSON.parse(fs.readFileSync(path.join(dir, "package.json"), "utf8"));
  const file = fs.readdirSync(dir).find((f) => /^(licen[cs]e|copying)(\.|$)/i.test(f));
  const text = file ? fs.readFileSync(path.join(dir, file), "utf8").trim() : `License: ${pkg.license}`;
  return `${pkg.name} ${pkg.version} (${pkg.license})\n\n${text}\n`;
});

fs.writeFileSync(path.join(out, "editor-licenses.txt"),
  "Third-party packages bundled into editor.js\n\n" + sections.join("\n" + "-".repeat(80) + "\n\n"));

const size = fs.statSync(path.join(out, "editor.js")).size;
console.log(`editor.js: ${(size / 1024).toFixed(0)} KB, ${packages.size} packages`);
