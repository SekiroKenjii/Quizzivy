import { resolve } from "node:path";
import { build } from "vite";
import { describe, expect, it, beforeAll } from "vitest";

// Rollup's types are only a transitive dependency, and Vite 8 bundles with
// rolldown, so the exported type names are not stable to import. Describing the
// two fields this test reads is more durable than either.
type OutputChunk = {
  type: "chunk";
  isEntry: boolean;
  fileName: string;
  moduleIds: readonly string[];
};
type BuildOutput = { output: ({ type: "asset" } | OutputChunk)[] };

/**
 * §2: "Split at the route level so a student never downloads admin code and an
 * anonymous visitor downloads neither."
 *
 * That is a claim about the built bundle, so it is checked against the built
 * bundle — not by reading the router and trusting that `lazy` did its job. A
 * refactor that turns one of those dynamic imports into a static one still
 * type-checks, still renders, and silently ships the admin tree to every
 * student. This is the only thing that would notice.
 */

// tests/integration -> tests -> web. Same depth as the old location,
// so the Vite root is still the web package.
const ROOT = resolve(import.meta.dirname, "../..");

let output: BuildOutput["output"];

beforeAll(async () => {
  const result = (await build({
    root: ROOT,
    configFile: resolve(ROOT, "vite.config.ts"),
    logLevel: "silent",
    build: { write: false },
  })) as unknown as BuildOutput | BuildOutput[];
  const first = Array.isArray(result) ? result[0]! : result;
  output = first.output;
}, 120_000);

const chunks = () => output.filter((o): o is OutputChunk => o.type === "chunk");
const entry = () => {
  const e = chunks().find((c) => c.isEntry);
  if (!e) throw new Error("no entry chunk in build output");
  return e;
};

/** Module ids belonging to a route tree, normalised to posix-ish paths. */
const matches = (ids: readonly string[], re: RegExp) =>
  ids.map((i) => i.replace(/\\/g, "/")).filter((i) => re.test(i));

const ADMIN =
  /\/(layouts\/AdminLayout|app\/pages\/AdminDashboardPage|features\/(tests|question-bank|media|students)\/)/;
const STUDENT =
  /\/(layouts\/StudentLayout|features\/assignments\/pages\/StudentHomePage)/;
const FOCUS = /\/layouts\/FocusLayout\./;

describe("route-level code splitting (§2)", () => {
  it("produces more than one chunk", () => {
    expect(chunks().length).toBeGreaterThan(3);
  });

  it("keeps the admin tree out of the entry chunk", () => {
    expect(
      matches(entry().moduleIds, ADMIN),
      "an anonymous visitor must not download admin code",
    ).toEqual([]);
  });

  it("keeps the student tree out of the entry chunk", () => {
    expect(matches(entry().moduleIds, STUDENT)).toEqual([]);
  });

  it("keeps the take-test shell out of the entry chunk", () => {
    expect(matches(entry().moduleIds, FOCUS)).toEqual([]);
  });

  it("still builds the admin tree, in non-entry chunks", () => {
    const owning = chunks().filter((c) => matches(c.moduleIds, ADMIN).length > 0);
    expect(owning.length, "admin modules must be built somewhere").toBeGreaterThan(0);
    expect(owning.map((c) => c.isEntry)).not.toContain(true);
  });

  it("does not let the admin and student trees share a chunk", () => {
    for (const c of chunks()) {
      const hasAdmin = matches(c.moduleIds, ADMIN).length > 0;
      const hasStudent = matches(c.moduleIds, STUDENT).length > 0;
      expect(hasAdmin && hasStudent, `chunk ${c.fileName} contains both trees`).toBe(
        false,
      );
    }
  });
});
