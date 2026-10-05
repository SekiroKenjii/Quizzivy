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
  imports: string[];
};
type BuildOutput = { output: ({ type: "asset" } | OutputChunk)[] };

/**
 * §2: "Split at the route level so a student never downloads admin code and an
 * anonymous visitor downloads neither."
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
  /\/(layouts\/AdminLayout|layouts\/Teacher(Layout|Shell|Skeleton)|features\/[^/]+\/pages\/teacher\/|app\/pages\/AdminDashboardPage|features\/(tests|question-bank|media|students)\/)/;
const STUDENT_PAGES = [
  "layouts/StudentLayout",
  "features/assignments/pages/StudentHomePage",
  "features/classes/pages/StudentClassesPage",
  "features/assignments/pages/AssignmentIntroPage",
  "features/results/pages/ResultPage",
  "features/auth/pages/StudentSettingsPage",
  "features/join/components/JoinDialog",
];
const STUDENT_ONLY = ["features/results/"];
const STUDENT = new RegExp(`/(${[...STUDENT_PAGES, ...STUDENT_ONLY].join("|")})`);
const FOCUS = /\/layouts\/FocusLayout\./;
const ENGINE = /\/features\/(take-test|integrity)\//;
const CLEARED_AT_SIGN_OUT = [
  "features/take-test/draft.ts",
  "features/take-test/groupPlaybackDraft.ts",
];

const DRAG = /\/@dnd-kit\//;
const SHELL = /\/src\/layouts\/TeacherShell\./;
const ROUTE_MODULE = /\/src\/((layouts|app\/pages)\/|features\/[^/]+\/pages\/)/;

const reachedFrom = (roots: readonly OutputChunk[]) => {
  const byName = new Map(chunks().map((chunk) => [chunk.fileName, chunk]));
  const seen = new Map<string, OutputChunk>();
  const pending = [...roots];
  while (pending.length) {
    const chunk = pending.pop()!;
    if (seen.has(chunk.fileName)) continue;
    seen.set(chunk.fileName, chunk);
    for (const name of chunk.imports) {
      const dependency = byName.get(name);
      if (dependency) pending.push(dependency);
    }
  }
  return [...seen.values()];
};

const eager = () => reachedFrom([entry()]);

describe("route-level code splitting (§2)", () => {
  it("keeps rich editors out of the entry and learner routes' static dependencies", () => {
    const byName = new Map(chunks().map((chunk) => [chunk.fileName, chunk]));
    const roots = chunks().filter(
      (chunk) =>
        chunk.isEntry ||
        matches(chunk.moduleIds, /\/features\/(take-test|results|assignments)\//)
          .length > 0,
    );
    const visited = new Set<string>();
    const pending = [...roots];
    while (pending.length) {
      const chunk = pending.pop()!;
      if (visited.has(chunk.fileName)) continue;
      visited.add(chunk.fileName);
      expect(
        matches(chunk.moduleIds, /@tiptap|prosemirror|parse5/),
        chunk.fileName,
      ).toEqual([]);
      for (const name of chunk.imports) {
        const dependency = byName.get(name);
        if (dependency) pending.push(dependency);
      }
    }
  });
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

  it("keeps the student tree and the take-test shell out of every chunk the entry loads statically", () => {
    expect(eager().length).toBeGreaterThan(1);
    for (const chunk of eager()) {
      expect(matches(chunk.moduleIds, STUDENT), chunk.fileName).toEqual([]);
      expect(matches(chunk.moduleIds, FOCUS), chunk.fileName).toEqual([]);
    }
  });

  it("loads nothing of the engine with the entry, save the two draft stores that signing out clears", () => {
    const loaded = eager()
      .flatMap((chunk) => matches(chunk.moduleIds, ENGINE))
      .map((id) => id.slice(id.lastIndexOf("/src/") + 5))
      .sort((a, b) => a.localeCompare(b));
    expect(loaded).toEqual(CLEARED_AT_SIGN_OUT);
  });

  it("still builds every student page, the take-test shell and the engine, in non-entry chunks", () => {
    const named = [
      ...STUDENT_PAGES,
      "layouts/FocusLayout",
      "features/take-test/pages/TakeTestPage",
    ];
    for (const page of named) {
      const owning = chunks().filter((chunk) =>
        chunk.moduleIds.some((id) =>
          id.replace(/\\/g, "/").includes(`/src/${page}.tsx`),
        ),
      );
      expect(owning.length, page).toBeGreaterThan(0);
      expect(
        owning.map((chunk) => chunk.isEntry),
        page,
      ).not.toContain(true);
    }
  });

  it("ships the system pages in the entry, so they render when a lazy chunk cannot load", () => {
    const ids = entry().moduleIds.map((i) => i.replace(/\\/g, "/"));
    for (const page of [
      "app/pages/NotFoundPage.tsx",
      "app/pages/ForbiddenPage.tsx",
      "app/pages/MaintenancePage.tsx",
      "app/ErrorBoundary.tsx",
      "app/pages/SystemFrame.tsx",
    ]) {
      expect(
        ids.some((id) => id.endsWith(`/src/${page}`)),
        page,
      ).toBe(true);
    }
  });

  it("ships the splash and the app states in the entry, so they cover a deep link while its route loads", () => {
    const ids = entry().moduleIds.map((i) => i.replace(/\\/g, "/"));
    for (const file of [
      "app/AppFrame.tsx",
      "app/boot/BootSplash.tsx",
      "app/boot/AppStateLayer.tsx",
      "app/boot/version.ts",
    ]) {
      expect(
        ids.some((id) => id.endsWith(`/src/${file}`)),
        file,
      ).toBe(true);
    }
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

  it("keeps the drag library behind the builder's own dynamic import", () => {
    const holding = chunks().filter(
      (chunk) => matches(chunk.moduleIds, DRAG).length > 0,
    );
    expect(holding.length, "the drag library must be built somewhere").toBeGreaterThan(
      0,
    );
    const shell = chunks().filter(
      (chunk) => matches(chunk.moduleIds, SHELL).length > 0,
    );
    expect(shell.length, "the teacher shell must be built somewhere").toBeGreaterThan(
      0,
    );
    const roots = chunks().filter(
      (chunk) => chunk.isEntry || matches(chunk.moduleIds, ROUTE_MODULE).length > 0,
    );
    for (const chunk of reachedFrom([...shell, ...roots])) {
      expect(matches(chunk.moduleIds, DRAG), chunk.fileName).toEqual([]);
    }
  });
});
