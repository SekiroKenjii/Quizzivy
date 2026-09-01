/* Turns the brand kit's SVGs into one injected sprite, verbatim.
   Usage: node build-brand-sprite.mjs <brand/svg dir> <out assets/brand.js> */
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const [dir, out] = process.argv.slice(2);

const MAP = [
  ["qz-mark-color", "quizzivy-mark-color.svg"],
  ["qz-mark-on-dark", "quizzivy-mark-on-dark.svg"],
  ["qz-mark-black", "quizzivy-mark-black.svg"],
  ["qz-mark-white", "quizzivy-mark-white.svg"],
  ["qz-logo-h-color", "quizzivy-logo-horizontal-color.svg"],
  ["qz-logo-h-on-dark", "quizzivy-logo-horizontal-on-dark.svg"],
  ["qz-logo-h-black", "quizzivy-logo-horizontal-black.svg"],
  ["qz-logo-h-white", "quizzivy-logo-horizontal-white.svg"],
  ["qz-logo-v-color", "quizzivy-logo-vertical-color.svg"],
  ["qz-logo-v-on-dark", "quizzivy-logo-vertical-on-dark.svg"],
  ["qz-logo-v-white", "quizzivy-logo-vertical-white.svg"],
  ["qz-appicon-light", "quizzivy-appicon-light.svg"],
  ["qz-appicon-dark", "quizzivy-appicon-dark.svg"],
  ["qz-favicon", "quizzivy-favicon.svg"],
];

const ratios = {};
const symbols = MAP.map(([id, file]) => {
  const raw = readFileSync(join(dir, file), "utf8");
  const viewBox = raw.match(/viewBox="([^"]+)"/)[1];
  const [, , w, h] = viewBox.split(/\s+/).map(Number);
  ratios[id] = +(w / h).toFixed(4);
  const inner = raw
    .replace(/^[\s\S]*?<svg[^>]*>/, "")
    .replace(/<\/svg>\s*$/, "")
    .replace(/\s+/g, " ")
    .trim();
  return `<symbol id="${id}" viewBox="${viewBox}">${inner}</symbol>`;
});

const sprite = `<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" style="position:absolute;width:0;height:0;overflow:hidden">${symbols.join("")}</svg>`;

writeFileSync(
  out,
  `/* Quizzivy brand kit as an inline sprite. Paths are Thuong's, copied verbatim from
   docs/design/brand/svg/ — do not hand-edit here; regenerate instead. */
const QUIZZIVY_BRAND_SPRITE = ${JSON.stringify(sprite)};

/** width / height of each symbol, so a caller can size by one dimension. */
const QUIZZIVY_BRAND_RATIO = ${JSON.stringify(ratios, null, 2)};

function mountBrandSprite() {
  if (document.getElementById("quizzivy-brand")) return;
  const host = document.createElement("div");
  host.id = "quizzivy-brand";
  host.setAttribute("aria-hidden", "true");
  host.innerHTML = QUIZZIVY_BRAND_SPRITE;
  document.body.insertBefore(host, document.body.firstChild);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", mountBrandSprite);
} else {
  mountBrandSprite();
}
`,
);
console.log("symbols:", symbols.length, "· sprite bytes:", sprite.length);
console.log(ratios);
