import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const CSS = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);

function unlayered(css: string) {
  let out = "";
  let depth = 0;
  let layered = 0;
  for (let i = 0; i < css.length; i++) {
    if (css.startsWith("@layer", i) && depth === 0) layered = 1;
    const c = css[i]!;
    if (c === "{") depth++;
    if (c === "}") {
      depth--;
      if (depth === 0) layered = 0;
    }
    if (layered === 0) out += c;
  }
  return out;
}

describe("one focus ring everywhere (DG-30)", () => {
  it("draws a 2px --focus outline, offset 2px, outside any cascade layer", () => {
    const rule =
      /(^|\n):focus-visible \{\s*outline: 2px solid var\(--focus\);\s*outline-offset: 2px;\s*\}/;
    expect(unlayered(CSS)).toMatch(rule);
  });
});
