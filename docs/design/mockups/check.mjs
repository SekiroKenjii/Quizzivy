/* Deck self-check: every icon referenced exists, every class has a rule.
   Run with `node docs/design/mockups/check.mjs` from anywhere. */
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(join(root, "assets/kit.css"), "utf8");
const icons = readFileSync(join(root, "assets/icons.js"), "utf8");

const defined = new Set();
for (const sel of css.matchAll(/\.((?:\\.|[-\w])+)/g)) defined.add(sel[1].replace(/\\/g, ""));
const symbols = new Set([...icons.matchAll(/id=\\"i-([a-z0-9-]+)\\"/g)].map((m) => m[1]));

/* Browsers recover from a stray close tag silently, so a sheet with one looks
   fine and stays wrong. Elements are walked with a stack; the first tag that
   closes something other than what is open is reported with its line. */
const VOID = new Set([
  "area", "base", "br", "col", "embed", "hr", "img", "input",
  "link", "meta", "param", "source", "track", "wbr",
]);

function unbalanced(html) {
  // Comments and script/style bodies are not markup.
  const src = html
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, " "))
    .replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1>/gi, (m) => m.replace(/[^\n]/g, " "));
  const line = (index) => src.slice(0, index).split("\n").length;
  const stack = [];
  const problems = [];
  for (const tag of src.matchAll(/<(\/?)([a-zA-Z][\w:-]*)\b[^>]*?(\/?)>/g)) {
    const [, closing, rawName, selfClosing] = tag;
    const name = rawName.toLowerCase();
    if (selfClosing || VOID.has(name)) continue;
    if (!closing) {
      stack.push({ name, line: line(tag.index) });
      continue;
    }
    const open = stack.pop();
    if (!open) {
      problems.push(`</${name}> at line ${line(tag.index)} closes nothing`);
    } else if (open.name !== name) {
      problems.push(
        `</${name}> at line ${line(tag.index)} closes <${open.name}> opened at line ${open.line}`,
      );
      break;
    }
  }
  if (problems.length === 0 && stack.length) {
    const open = stack[stack.length - 1];
    problems.push(`<${open.name}> opened at line ${open.line} is never closed`);
  }
  return problems;
}

const files = [
  "index.html",
  ...readdirSync(join(root, "sheets"))
    .filter((f) => f.endsWith(".html") && !f.startsWith("_"))
    .map((f) => join("sheets", f)),
];

let failures = 0;
for (const file of files) {
  const html = readFileSync(join(root, file), "utf8");

  const usedIcons = new Set([...html.matchAll(/href="#i-([a-z0-9-]+)"/g)].map((m) => m[1]));
  const missingIcons = [...usedIcons].filter((n) => !symbols.has(n));

  const usedClasses = new Set();
  for (const attr of html.matchAll(/class="([^"]+)"/g)) {
    for (const c of attr[1].trim().split(/\s+/)) usedClasses.add(c);
  }
  const missingClasses = [...usedClasses].filter((c) => !defined.has(c));
  const nesting = unbalanced(html);

  const status =
    missingIcons.length + missingClasses.length + nesting.length === 0 ? "ok" : "FAIL";
  if (status === "FAIL") failures++;
  console.log(
    `${status.padEnd(4)} ${file.padEnd(34)} ${usedClasses.size} classes, ${usedIcons.size} icons`,
  );
  if (missingIcons.length) console.log(`     missing icons:   ${missingIcons.join(", ")}`);
  if (missingClasses.length) console.log(`     undefined classes: ${missingClasses.join(", ")}`);
  for (const problem of nesting) console.log(`     nesting: ${problem}`);
}

console.log(failures ? `\n${failures} file(s) with problems` : "\nall sheets clean");
process.exit(failures ? 1 : 0);
