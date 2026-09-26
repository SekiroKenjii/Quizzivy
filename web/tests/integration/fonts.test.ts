import { resolve } from "node:path";
import { build } from "vite";
import { beforeAll, describe, expect, it } from "vitest";

type Output = {
  type: "asset" | "chunk";
  fileName: string;
  source?: string | Uint8Array;
  code?: string;
};

const ROOT = resolve(import.meta.dirname, "../..");
let output: Output[];

beforeAll(async () => {
  const result = (await build({
    root: ROOT,
    configFile: resolve(ROOT, "vite.config.ts"),
    logLevel: "silent",
    build: { write: false },
  })) as unknown as { output: Output[] } | { output: Output[] }[];
  output = (Array.isArray(result) ? result[0]! : result).output;
}, 120_000);

const text = (o: Output) => o.code ?? (typeof o.source === "string" ? o.source : "");

describe("Be Vietnam Pro is self-hosted (spec §12, CSP font-src 'self')", () => {
  it("references fonts only as built assets", () => {
    const css = output
      .filter((o) => o.fileName.endsWith(".css"))
      .map(text)
      .join("\n");
    const urls = [...css.matchAll(/url\(([^)]+\.woff2?)\)/g)].map((m) => m[1]!);
    expect(urls.length).toBeGreaterThan(0);
    expect(urls.filter((u) => !u.startsWith("/assets/"))).toEqual([]);
    expect(css).toContain("Be Vietnam Pro");
    expect(css).not.toMatch(/font-family:[^;]*\bInter\b/);
  });

  it("emits the Vietnamese subset for every weight the deck uses", () => {
    const woff2 = output.map((o) => o.fileName).filter((f) => f.endsWith(".woff2"));
    for (const weight of [400, 500, 600, 700]) {
      expect(woff2.some((f) => f.includes(`vietnamese-${weight}-normal`))).toBe(true);
      expect(woff2.some((f) => f.includes(`latin-${weight}-normal`))).toBe(true);
    }
  });

  it("never reaches for Google Fonts", () => {
    const leaks = output.filter((o) =>
      /fonts\.(googleapis|gstatic)\.com/.test(text(o)),
    );
    expect(leaks.map((o) => o.fileName)).toEqual([]);
  });
});
