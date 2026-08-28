import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The half of §14 that `react/jsx-no-literals` cannot cover.
 *
 * That rule checks JSX text children. It is configured with `ignoreProps`,
 * because checking every attribute would flag `className`, `id` and `type` and
 * be unusable. But some attributes ARE user-facing — an `aria-label` is what a
 * screen-reader user hears, and a `placeholder` is read as ordinary copy. An
 * English one of those in front of a Vietnamese student is exactly what
 * AGENTS.md forbids.
 *
 * So this checks precisely the attributes that carry human-readable text.
 */

const SRC = resolve(import.meta.dirname, "../../../src");
const USER_FACING_ATTRS = [
  "aria-label",
  "aria-description",
  "placeholder",
  "title",
  "alt",
];

function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (entry === "node_modules" || entry === "api") return [];
    if (statSync(full).isDirectory()) return tsxFiles(full);
    return full.endsWith(".tsx") ? [full] : [];
  });
}

describe("§14: no hardcoded user-facing strings", () => {
  it("has no literal text in user-facing attributes", () => {
    const offenders: string[] = [];

    for (const file of tsxFiles(SRC)) {
      const source = readFileSync(file, "utf8");
      source.split("\n").forEach((line, i) => {
        for (const attr of USER_FACING_ATTRS) {
          // Matches attr="literal text" but not attr={t("key")}.
          const re = new RegExp(`\\b${attr}\\s*=\\s*"([^"]+)"`, "g");
          for (const m of line.matchAll(re)) {
            const value = m[1]!;
            // Empty alt="" is a deliberate a11y signal for decorative images.
            if (attr === "alt" && value === "") continue;
            offenders.push(`${relative(SRC, file)}:${i + 1}  ${attr}="${value}"`);
          }
        }
      });
    }

    expect(
      offenders,
      "these must come from t() — an English aria-label is read aloud to a Vietnamese student",
    ).toEqual([]);
  });
});
