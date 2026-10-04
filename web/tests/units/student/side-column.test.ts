import { beforeEach, describe, expect, it, vi } from "vitest";

const loaded = vi.hoisted(() => ({ sideColumn: false }));

vi.mock("@/components/shared/SideColumn", () => {
  loaded.sideColumn = true;
  return { SideColumn: () => null };
});

interface RouteNode {
  path?: string | undefined;
  lazy?: unknown;
  children?: RouteNode[] | undefined;
}

interface Walked {
  path: string;
  load: (() => Promise<{ Component?: unknown }>) | null;
}

function walk(node: RouteNode, base: string, out: Walked[]): Walked[] {
  const path = [base, node.path].filter(Boolean).join("/");
  out.push({
    path,
    load:
      typeof node.lazy === "function"
        ? (node.lazy as () => Promise<{ Component?: unknown }>)
        : null,
  });
  for (const child of node.children ?? []) walk(child, path, out);
  return out;
}

function under(prefix: string, nodes: RouteNode[], out: Walked[] = []): Walked[] {
  for (const node of nodes) {
    if (node.path?.split("/")[0] === prefix) walk(node, "", out);
    else under(prefix, node.children ?? [], out);
  }
  return out;
}

async function routesUnder(prefix: string) {
  const { router } = await import("@/app/router");
  return under(prefix, router.routes);
}

beforeEach(() => {
  vi.resetModules();
  loaded.sideColumn = false;
});

describe("the student tree", () => {
  it("loads no SideColumn for any /app route, the engine included", async () => {
    const routes = await routesUnder("app");

    expect([...new Set(routes.map((route) => route.path))]).toEqual(
      expect.arrayContaining([
        "app",
        "app/classes",
        "app/assignments/:id",
        "app/settings/:section?",
        "app/attempts/:attemptId/result",
        "app/attempts/:attemptId",
      ]),
    );
    const pages = routes.flatMap((route) => (route.load === null ? [] : [route.load]));
    expect(pages.length).toBeGreaterThanOrEqual(8);
    for (const load of pages) {
      expect((await load()).Component).toBeTypeOf("function");
    }

    expect(loaded.sideColumn).toBe(false);
  }, 60_000);

  it("would be caught: the teacher's layout, walked the same way, loads one", async () => {
    const routes = await routesUnder("admin");
    const layout = routes.find((route) => route.load !== null);
    expect(layout?.path).toBe("admin");
    expect(loaded.sideColumn).toBe(false);

    await layout?.load?.();
    expect(loaded.sideColumn).toBe(true);
  }, 60_000);
});
