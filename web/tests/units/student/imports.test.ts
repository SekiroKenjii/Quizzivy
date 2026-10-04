import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const ROUTER = readFileSync(resolve(SRC, "app/router.tsx"), "utf8");
const SIDE_COLUMNS = [
  "components/shared/PageAside.tsx",
  "components/shared/SideColumn.tsx",
  "hooks/useColumnWidth.ts",
];

function fileOf(specifier: string, from: string): string | null {
  let base: string;
  if (specifier.startsWith("@/")) base = resolve(SRC, specifier.slice(2));
  else if (specifier.startsWith(".")) base = resolve(dirname(from), specifier);
  else return null;
  for (const tail of ["", ".ts", ".tsx", "/index.ts", "/index.tsx"]) {
    const path = base + tail;
    if (existsSync(path) && statSync(path).isFile()) return path;
  }
  return null;
}

function importsOf(path: string): string[] {
  if (!/\.tsx?$/.test(path)) return [];
  const source = readFileSync(path, "utf8");
  const specifiers = [
    ...source.matchAll(/\bfrom\s*["']([^"']+)["']/g),
    ...source.matchAll(/\bimport\s*\(\s*["'`]([^"'`]+)["'`]\s*\)/g),
    ...source.matchAll(/\bimport\s*["']([^"']+)["']/g),
  ].map((match) => match[1]!);
  return specifiers.flatMap((specifier) => {
    const file = fileOf(specifier, path);
    return file === null ? [] : [file];
  });
}

function routesOf(tree: string): string[] {
  const start = ROUTER.indexOf(`const ${tree}: RouteObject = {`);
  const end = ROUTER.indexOf("\n};", start);
  if (start < 0 || end < 0) throw new Error(`no ${tree} in the router`);
  return [...ROUTER.slice(start, end).matchAll(/import\("([^"]+)"\)/g)].map((match) =>
    fileOf(match[1]!, resolve(SRC, "app/router.tsx"))!,
  );
}

function reachable(roots: string[]): Set<string> {
  const seen = new Set<string>();
  const pending = [...roots];
  while (pending.length > 0) {
    const path = pending.pop()!;
    if (seen.has(path)) continue;
    seen.add(path);
    pending.push(...importsOf(path));
  }
  return seen;
}

const named = (paths: Iterable<string>) =>
  [...paths].map((path) => relative(SRC, path)).sort((a, b) => a.localeCompare(b));

describe("what the student routes import", () => {
  const student = [...routesOf("studentTree"), ...routesOf("takeTestTree")];

  it("walks every route the router declares under /app", () => {
    const declared = ROUTER.matchAll(
      /(?:const (\w+): RouteObject = \{\s*)?\bpath: "app[/"]/g,
    );
    expect([...declared].map((match) => match[1])).toEqual([
      "studentTree",
      "takeTestTree",
    ]);
  });

  it("starts from the shell, its pages and the engine", () => {
    expect(named(student)).toEqual(
      expect.arrayContaining([
        "features/assignments/pages/AssignmentIntroPage.tsx",
        "features/assignments/pages/StudentHomePage.tsx",
        "features/auth/pages/StudentSettingsPage.tsx",
        "features/classes/pages/StudentClassesPage.tsx",
        "features/results/pages/ResultPage.tsx",
        "features/take-test/pages/TakeTestPage.tsx",
        "layouts/FocusLayout.tsx",
        "layouts/StudentLayout.tsx",
      ]),
    );
  });

  it("reaches no PageAside, SideColumn or useColumnWidth, through any module", () => {
    const modules = reachable(student);
    expect(modules.size).toBeGreaterThan(student.length);
    expect(named(modules).filter((path) => SIDE_COLUMNS.includes(path))).toEqual([]);
  });

  it("would be caught: the teacher's routes, walked the same way, reach all three", () => {
    const modules = named(reachable(routesOf("adminTree")));
    expect(modules.filter((path) => SIDE_COLUMNS.includes(path))).toEqual(SIDE_COLUMNS);
  });

  it("names three modules that exist", () => {
    for (const path of SIDE_COLUMNS) {
      expect(existsSync(resolve(SRC, path)), path).toBe(true);
    }
  });
});
