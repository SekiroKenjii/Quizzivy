/* Bundles the deck into one self-contained page for sharing.
   Usage: node docs/design/mockups/build-artifact.mjs <out.html> */
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const out = process.argv[2] || join(root, "deck.html");

const PANELS = [
  { id: "overview", label: "Overview", file: "index.html" },
  { id: "foundations", label: "Foundations", file: "sheets/00-foundations.html" },
  { id: "student", label: "Student", file: "sheets/10-student.html" },
  { id: "authoring", label: "Authoring", file: "sheets/20-teacher-authoring.html" },
  { id: "grading", label: "Assign & grade", file: "sheets/30-teacher-assign-grade.html" },
  { id: "lms", label: "LMS vision", file: "sheets/40-lms-future.html" },
];

/** The `<div class="deck-wrap">…</div>` body of a sheet, without the deck chrome around it. */
function extractBody(html) {
  const open = html.indexOf('<div class="deck-wrap"');
  const start = html.indexOf(">", open) + 1;
  const end = html.indexOf("<script", start);
  return html.slice(start, end).replace(/\s*<\/div>\s*$/, "");
}

/* Route the index cards and any cross-sheet link through the panel switcher. */
function rewriteLinks(html) {
  const byFile = Object.fromEntries(PANELS.map((p) => [p.file.split("/").pop(), p.id]));
  return html.replace(/href="(?:sheets\/)?([\w.-]+\.html)"/g, (m, file) =>
    byFile[file] ? `data-goto="${byFile[file]}" role="button" tabindex="0"` : m,
  );
}

let css = readFileSync(join(root, "assets/kit.css"), "utf8").replace(/^@import url\([^)]*\);\s*/, "");

/* The kit themes off a `.dark` class; an artifact themes off the viewer's three states.
   Same token block, three more selectors — no component rule changes. */
const darkBlock = css.match(/\.dark \{([\s\S]*?)\n\}/)[1];
css += `
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {${darkBlock}
  }
}
:root[data-theme="dark"] {${darkBlock}
}
`;

const icons = readFileSync(join(root, "assets/icons.js"), "utf8");
const sprite = JSON.parse(icons.match(/const QUIZZIVY_ICON_SPRITE = (".*?");\n/s)[1]);

const nav = PANELS.map(
  (p, i) =>
    `<button class="btn btn-sm ${i === 0 ? "btn-secondary" : "btn-ghost"}" data-goto="${p.id}">${p.label}</button>`,
).join("\n        ");

const panels = PANELS.map((p) => {
  const body = rewriteLinks(extractBody(readFileSync(join(root, p.file), "utf8")));
  const wide = p.id === "overview" ? ' style="max-width: 68rem"' : "";
  return `<section class="deck-wrap panel"${wide} id="panel-${p.id}"${p.id === "overview" ? "" : " hidden"}>${body}</section>`;
}).join("\n");

writeFileSync(
  out,
  `<title>Quizzivy Design Deck</title>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" />
<style>
${css}
/* single-page shell */
body { background: var(--deck-canvas); }
.panel[hidden] { display: none; }
</style>
${sprite}
<div class="deck-bar">
  <span class="font-semibold tracking-tight">Quizzivy</span>
  <span class="text-xs text-muted-foreground">design deck</span>
  <nav class="flex items-center gap-1 ml-4">
    ${nav}
  </nav>
  <div class="ml-auto flex items-center gap-1 border rounded-md px-1 h-8">
    <button class="btn btn-xs btn-ghost" data-zoom="-0.1" aria-label="Zoom out">−</button>
    <span class="text-xs text-muted-foreground tabular-nums w-10 text-center" data-zoom-label>100%</span>
    <button class="btn btn-xs btn-ghost" data-zoom="0.1" aria-label="Zoom in">+</button>
  </div>
</div>
${panels}
<script>
(function () {
  function show(id) {
    document.querySelectorAll(".panel").forEach(function (p) {
      p.hidden = p.id !== "panel-" + id;
    });
    document.querySelectorAll(".deck-bar [data-goto]").forEach(function (b) {
      var on = b.dataset.goto === id;
      b.classList.toggle("btn-secondary", on);
      b.classList.toggle("btn-ghost", !on);
    });
    scrollTo({ top: 0 });
  }

  function applyZoom(z) {
    document.querySelectorAll(".frame-desktop, .frame-wide, .frame-tablet").forEach(function (el) {
      el.style.zoom = String(z);
    });
    document.querySelectorAll("[data-zoom-label]").forEach(function (el) {
      el.textContent = Math.round(z * 100) + "%";
    });
    zoom = z;
  }

  var zoom = 1;

  addEventListener("click", function (e) {
    var goto = e.target.closest("[data-goto]");
    if (goto) { e.preventDefault(); show(goto.dataset.goto); return; }
    var z = e.target.closest("[data-zoom]");
    if (z) applyZoom(Math.min(1, Math.max(0.5, Math.round((zoom + Number(z.dataset.zoom)) * 20) / 20)));
  });

  addEventListener("keydown", function (e) {
    if (e.key !== "Enter" && e.key !== " ") return;
    var goto = e.target.closest && e.target.closest("[data-goto]");
    if (goto) { e.preventDefault(); show(goto.dataset.goto); }
  });
})();
<\/script>
`,
);
console.log("wrote", out);
