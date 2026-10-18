import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx$/.test(name) ? [path] : [];
  });
}

const searchesInTheUrl = sources(SRC).filter((path) => {
  const text = readFileSync(path, "utf8");
  return text.includes("<SearchInput") && text.includes('"q"');
});

describe("a search box whose text lives in the URL", () => {
  it("is found where the lists are", () => {
    expect(searchesInTheUrl.length).toBeGreaterThanOrEqual(10);
  });

  it.each(searchesInTheUrl.map((path) => relative(SRC, path)))(
    "%s holds its text through useDebouncedInput, so a slow navigation cannot take a keystroke back",
    (path) => {
      expect(readFileSync(join(SRC, path), "utf8")).toContain("useDebouncedInput(");
    },
  );
});
