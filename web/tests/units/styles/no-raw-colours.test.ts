import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const ALLOWED = new Set(["features/auth/components/GoogleMark.tsx"]);

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

const UTILITIES = [
  "bg",
  "text",
  "border",
  "ring",
  "fill",
  "stroke",
  "outline",
  "from",
  "via",
  "to",
  "divide",
  "placeholder",
  "decoration",
  "shadow",
  "caret",
  "accent",
];
const TAILWIND_COLOURS = [
  "zinc",
  "slate",
  "gray",
  "neutral",
  "stone",
  "red",
  "orange",
  "amber",
  "yellow",
  "lime",
  "green",
  "emerald",
  "teal",
  "cyan",
  "sky",
  "blue",
  "indigo",
  "violet",
  "purple",
  "fuchsia",
  "pink",
  "rose",
  "white",
  "black",
];
const PALETTE = new RegExp(
  String.raw`\b(?:${UTILITIES.join("|")})-(?:${TAILWIND_COLOURS.join("|")})\b`,
);
const LITERAL = /#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b|(?<![A-Za-z])(?:rgba?|oklch)\(/;

describe("colours come from tokens (spec §12)", () => {
  it("has no raw colour in application code", () => {
    const offenders = sources(SRC).flatMap((path) => {
      const file = relative(SRC, path).replaceAll("\\", "/");
      if (ALLOWED.has(file)) return [];
      return readFileSync(path, "utf8")
        .split("\n")
        .flatMap((line, index) =>
          PALETTE.test(line) || LITERAL.test(line)
            ? [`${file}:${index + 1}: ${line.trim()}`]
            : [],
        );
    });
    expect(offenders).toEqual([]);
  });
});
