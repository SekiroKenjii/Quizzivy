import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";

import { coveredCodePoints, fontFaces } from "@tests/support/fontCoverage";

const SRC = resolve(import.meta.dirname, "../../../src");
const LOCALES = join(SRC, "lib/i18n/locales");
const IGNORABLE = /[\p{Cc}\p{Cf}\p{Zs}\p{Zl}\p{Zp}]/u;

/**
 * Files whose strings render only in a monospace face or never reach the
 * screen, each with the characters it may use outside Be Vietnam Pro.
 */
const ALLOWED: Record<string, string> = {
  "features/notifications/kinds.ts": "\uE000\uE001",
};

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) && !name.endsWith(".d.ts") ? [path] : [];
  });
}

function sourceStrings(path: string): Array<{ line: number; text: string }> {
  const file = ts.createSourceFile(
    path,
    readFileSync(path, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
  const out: Array<{ line: number; text: string }> = [];
  const visit = (node: ts.Node) => {
    if (
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node) ||
      ts.isTemplateHead(node) ||
      ts.isTemplateMiddle(node) ||
      ts.isTemplateTail(node) ||
      ts.isJsxText(node)
    ) {
      const line = file.getLineAndCharacterOfPosition(node.getStart()).line + 1;
      out.push({ line, text: node.text });
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return out;
}

function localeStrings(value: unknown, key = ""): Array<{ key: string; text: string }> {
  if (typeof value === "string") return [{ key, text: value }];
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([k, v]) =>
      localeStrings(v, key ? `${key}.${k}` : k),
    );
  }
  return [];
}

function codePoint(c: string): string {
  return `U+${c.codePointAt(0)!.toString(16).toUpperCase().padStart(4, "0")} ${c}`;
}

function uncovered(text: string, covered: Set<number>, allowed = ""): string[] {
  return [...new Set(text)].filter(
    (c) =>
      !covered.has(c.codePointAt(0)!) && !IGNORABLE.test(c) && !allowed.includes(c),
  );
}

describe("every UI character renders in Be Vietnam Pro (spec §12)", () => {
  const covered = coveredCodePoints("normal");

  it("reads the loaded faces' coverage from the installed font", () => {
    expect(covered.has("ệ".codePointAt(0)!)).toBe(true);
    expect(covered.has("–".codePointAt(0)!)).toBe(true);
    expect(covered.has("→".codePointAt(0)!)).toBe(false);
    expect(covered.has(0x0302)).toBe(false);
  });

  it("loads an italic face for every upright weight, covering the same characters", () => {
    const weights = (style: string) =>
      [
        ...new Set(
          fontFaces()
            .filter((f) => f.style === style)
            .map((f) => f.weight),
        ),
      ].sort((a, b) => a.localeCompare(b));
    expect(weights("italic")).toEqual(weights("normal"));
    const italic = coveredCodePoints("italic");
    expect([...covered].filter((c) => !italic.has(c))).toEqual([]);
  });

  it("has no string in the source outside the loaded fonts", () => {
    const offenders = sources(SRC).flatMap((path) => {
      const file = relative(SRC, path).replaceAll("\\", "/");
      return sourceStrings(path).flatMap(({ line, text }) =>
        uncovered(text, covered, ALLOWED[file]).map(
          (c) => `${file}:${line}: ${codePoint(c)}`,
        ),
      );
    });
    expect(offenders).toEqual([]);
  });

  it("has no locale string outside the loaded fonts", () => {
    const offenders = readdirSync(LOCALES).flatMap((name) => {
      const json: unknown = JSON.parse(readFileSync(join(LOCALES, name), "utf8"));
      return localeStrings(json).flatMap(({ key, text }) =>
        uncovered(text, covered).map((c) => `${name} ${key}: ${codePoint(c)}`),
      );
    });
    expect(offenders).toEqual([]);
  });
});
