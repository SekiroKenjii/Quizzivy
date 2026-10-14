import { describe, expect, it } from "vitest";
import type { RouteObject } from "react-router";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { router } from "@/app/router";
import { ADMIN_NAV, type AdminNavItem } from "@/layouts/adminNav";
import { loadSpec, operations } from "@tests/support/openapi";

const OPERATIONS = new Map(
  operations(loadSpec()).map((operation) => [
    operation.op.operationId as string,
    operation,
  ]),
);
const ITEMS = ADMIN_NAV.flatMap((group) => group.items);

const NOT_IN_THE_CONTRACT_YET = new Map<string, string>([
  ["getAdminOverview", "T-R5.18"],
  ["listUsers", "T-R5.9a"],
  ["listAllClasses", "T-R5.11"],
]);

function comparable(permission: unknown): string {
  return JSON.stringify(
    Array.isArray(permission)
      ? [...permission].sort((a, b) => String(a).localeCompare(String(b)))
      : permission,
  );
}

function disagreements(
  items: readonly AdminNavItem[],
  pending: ReadonlyMap<string, string> = NOT_IN_THE_CONTRACT_YET,
): string[] {
  return items.flatMap((item) => {
    const found = OPERATIONS.get(item.operation);
    if (found === undefined)
      return pending.has(item.operation)
        ? []
        : [`${item.id}: no operation ${item.operation}`];
    const problems: string[] = [];
    if (pending.has(item.operation))
      problems.push(
        `${item.id}: ${item.operation} is in the contract now, drop it from the pending list`,
      );
    if (found.method !== "get" || !found.path.startsWith("/admin"))
      problems.push(`${item.id}: ${item.operation} is ${found.method} ${found.path}`);
    if (comparable(found.op["x-permission"]) !== comparable(item.requires))
      problems.push(
        `${item.id}: requires ${comparable(item.requires)}, ${item.operation} declares ${comparable(found.op["x-permission"])}`,
      );
    return problems;
  });
}

function adminChildren(): RouteObject[] {
  const tree = router.routes.flatMap(function find(route): RouteObject[] {
    if (route.path === "admin") return [route];
    return (route.children ?? []).flatMap(find);
  })[0];
  const layout = tree?.children?.[0]?.children?.[0];
  if (!layout?.children) throw new Error("no admin tree with a layout in the router");
  return layout.children;
}

function routeFor(address: string): RouteObject | undefined {
  const path =
    address === ADMIN_PATHS.overview ? undefined : address.slice("/admin/".length);
  return adminChildren().find((route) =>
    path === undefined ? route.index === true : route.path?.split("/")[0] === path,
  );
}

describe("the Admin sidebar against the contract", () => {
  it("opens, with each destination, a read the contract has or a later task adds, under the permission it declares", () => {
    expect(ITEMS).toHaveLength(8);
    expect(disagreements(ITEMS)).toEqual([]);
  });

  it("has an operation that exists today, and it agrees", () => {
    const today = ITEMS.filter((item) => OPERATIONS.has(item.operation));
    expect(today.map((item) => item.id)).toContain("ai");
    expect(disagreements(today)).toEqual([]);
  });

  it("would be caught: AI credits behind the workspace alone", () => {
    const loosened = ITEMS.map((item) =>
      item.id === "ai" ? { ...item, requires: "workspace.admin" as const } : item,
    );
    expect(disagreements(loosened)).toEqual([
      'ai: requires "workspace.admin", getAiCredits declares "ai.credits.manage"',
    ]);
  });

  it("would be caught: a key widened to a list, an operation that is not an admin read, a name nobody adds", () => {
    const changed = ITEMS.map((item): AdminNavItem => {
      if (item.id === "ai") return { ...item, requires: ["ai.credits.manage"] };
      if (item.id === "audit") return { ...item, operation: "getAiCredits" };
      if (item.id === "api") return { ...item, operation: "listEverything" };
      if (item.id === "settings")
        return { ...item, operation: "approveAiCreditRequest" };
      return item;
    });
    expect(disagreements(changed)).toEqual([
      'ai: requires ["ai.credits.manage"], getAiCredits declares "ai.credits.manage"',
      'audit: requires "system.audit.read", getAiCredits declares "ai.credits.manage"',
      "api: no operation listEverything",
      "settings: approveAiCreditRequest is post /admin/ai/credit-requests/{id}/approve",
      'settings: requires "system.settings.write", approveAiCreditRequest declares "ai.credits.manage"',
    ]);
  });

  it("would be caught: an operation the contract has since gained, still on the pending list", () => {
    const pending = new Map(NOT_IN_THE_CONTRACT_YET).set("getAiCredits", "T-R5.35");
    expect(disagreements(ITEMS, pending)).toEqual([
      "ai: getAiCredits is in the contract now, drop it from the pending list",
    ]);
  });

  it("gives every destination its own address, each a route of the Admin tree under the same requirement", () => {
    const addresses = ITEMS.map((item) => item.to);
    expect(new Set(addresses).size).toBe(addresses.length);
    expect(addresses).toEqual(Object.values(ADMIN_PATHS));
    for (const item of ITEMS) {
      const route = routeFor(item.to);
      expect(route, item.id).toBeDefined();
      expect((route!.handle as { permission: unknown }).permission, item.id).toEqual(
        item.requires,
      );
    }
  });

  it("declares a crumb, a width and a permission on every route of the tree", () => {
    const routes = adminChildren();
    expect(routes.map((route) => route.path ?? "(index)")).toEqual([
      "(index)",
      "users",
      "roles",
      "classes",
      "ai",
      "audit",
      "api",
      "settings/:section?",
      "exports/:id",
    ]);
    for (const route of routes) {
      const handle = route.handle as Record<string, unknown>;
      expect(Array.isArray(handle["crumb"]), route.path).toBe(true);
      expect([1080, 1320], route.path).toContain(handle["width"]);
      expect(handle["permission"], route.path).toBeDefined();
    }
  });

  it("gives API reference and Settings the narrow frame, the rest the wide one", () => {
    const widths = Object.fromEntries(
      adminChildren().map((route) => [
        route.path ?? "(index)",
        (route.handle as { width: number }).width,
      ]),
    );
    expect(widths).toEqual({
      "(index)": 1320,
      users: 1320,
      roles: 1320,
      classes: 1320,
      ai: 1320,
      audit: 1320,
      api: 1080,
      "settings/:section?": 1080,
      "exports/:id": 1080,
    });
  });
});
