import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { inflateSync } from "node:zlib";

const WEB = resolve(import.meta.dirname, "../..");
const INDEX_CSS = resolve(WEB, "src/index.css");

/** FontFace is one `@font-face` rule of an imported stylesheet. */
export interface FontFace {
  css: string;
  style: string;
  weight: string;
  woff: string;
  ranges: Array<[number, number]>;
}

function packageImports(css: string): string[] {
  return [...css.matchAll(/@import\s+"(@fontsource\/[^"]+\.css)"/g)].map((m) => m[1]!);
}

function parseRanges(value: string): Array<[number, number]> {
  return value.split(",").map((part) => {
    const [from, to] = part.trim().replace(/^U\+/i, "").split("-");
    return [parseInt(from!, 16), parseInt(to ?? from!, 16)];
  });
}

/** fontFaces reads every `@font-face` that `src/index.css` imports from fontsource. */
export function fontFaces(): FontFace[] {
  const index = readFileSync(INDEX_CSS, "utf8");
  return packageImports(index).flatMap((spec) => {
    const path = resolve(WEB, "node_modules", spec);
    const css = readFileSync(path, "utf8");
    return [...css.matchAll(/@font-face\s*{([^}]*)}/g)].map(([, body]) => {
      const field = (name: string) =>
        new RegExp(`${name}:\\s*([^;]+);`).exec(body!)?.[1]?.trim() ?? "";
      const woff = /url\(([^)]+\.woff)\)\s*format\('woff'\)/.exec(body!)?.[1];
      if (!woff) throw new Error(`${spec}: a face without a WOFF source`);
      return {
        css: spec,
        style: field("font-style"),
        weight: field("font-weight"),
        woff: resolve(dirname(path), woff),
        ranges: parseRanges(field("unicode-range")),
      };
    });
  });
}

function cmapTable(woff: Buffer): Buffer {
  if (woff.toString("latin1", 0, 4) !== "wOFF") throw new Error("not a WOFF file");
  const tables = woff.readUInt16BE(12);
  for (let i = 0; i < tables; i++) {
    const entry = 44 + i * 20;
    if (woff.toString("latin1", entry, entry + 4) !== "cmap") continue;
    const offset = woff.readUInt32BE(entry + 4);
    const compressed = woff.readUInt32BE(entry + 8);
    const original = woff.readUInt32BE(entry + 12);
    const data = woff.subarray(offset, offset + compressed);
    return compressed < original ? inflateSync(data) : data;
  }
  throw new Error("no cmap table");
}

function format4(cmap: Buffer, at: number, out: Set<number>): void {
  const segments = cmap.readUInt16BE(at + 6) / 2;
  const ends = at + 14;
  const starts = ends + segments * 2 + 2;
  const deltas = starts + segments * 2;
  const rangeOffsets = deltas + segments * 2;
  for (let s = 0; s < segments; s++) {
    const end = cmap.readUInt16BE(ends + s * 2);
    const start = cmap.readUInt16BE(starts + s * 2);
    const delta = cmap.readInt16BE(deltas + s * 2);
    const rangeOffset = cmap.readUInt16BE(rangeOffsets + s * 2);
    for (let c = start; c <= end && c !== 0xffff; c++) {
      let glyph: number;
      if (rangeOffset === 0) {
        glyph = (c + delta) & 0xffff;
      } else {
        const index = rangeOffsets + s * 2 + rangeOffset + (c - start) * 2;
        glyph = cmap.readUInt16BE(index);
        if (glyph !== 0) glyph = (glyph + delta) & 0xffff;
      }
      if (glyph !== 0) out.add(c);
    }
  }
}

function format12(cmap: Buffer, at: number, out: Set<number>): void {
  const groups = cmap.readUInt32BE(at + 12);
  for (let g = 0; g < groups; g++) {
    const group = at + 16 + g * 12;
    const start = cmap.readUInt32BE(group);
    const end = cmap.readUInt32BE(group + 4);
    const glyph = cmap.readUInt32BE(group + 8);
    for (let c = start; c <= end; c++) if (glyph + (c - start) !== 0) out.add(c);
  }
}

/** glyphs returns the code points a WOFF file maps to a glyph. */
export function glyphs(woffPath: string): Set<number> {
  const cmap = cmapTable(readFileSync(woffPath));
  const out = new Set<number>();
  const records = cmap.readUInt16BE(2);
  for (let r = 0; r < records; r++) {
    const platform = cmap.readUInt16BE(4 + r * 8);
    const encoding = cmap.readUInt16BE(6 + r * 8);
    if (!(platform === 0 || (platform === 3 && (encoding === 1 || encoding === 10))))
      continue;
    const at = cmap.readUInt32BE(8 + r * 8);
    const format = cmap.readUInt16BE(at);
    if (format === 4) format4(cmap, at, out);
    if (format === 12) format12(cmap, at, out);
  }
  return out;
}

/**
 * coveredCodePoints returns the code points every loaded weight of one style
 * renders: those inside a face's `unicode-range` that its file also has a glyph
 * for. A character the range names but the file lacks still falls back.
 */
export function coveredCodePoints(style = "normal"): Set<number> {
  const byWeight = new Map<string, Set<number>>();
  for (const face of fontFaces().filter((f) => f.style === style)) {
    const covered = byWeight.get(face.weight) ?? new Set<number>();
    const inFile = glyphs(face.woff);
    for (const [from, to] of face.ranges) {
      for (let c = from; c <= to; c++) if (inFile.has(c)) covered.add(c);
    }
    byWeight.set(face.weight, covered);
  }
  const [first, ...rest] = [...byWeight.values()];
  if (!first) return new Set();
  return new Set([...first].filter((c) => rest.every((weight) => weight.has(c))));
}
