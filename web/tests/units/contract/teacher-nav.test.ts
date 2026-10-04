import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { TEACHER_NAV, type TeacherNavItem } from "@/layouts/teacherNav";
import { loadSpec, operations } from "@tests/support/openapi";

const OPERATIONS = new Map(
  operations(loadSpec()).map((operation) => [
    operation.op.operationId as string,
    operation,
  ]),
);
const ITEMS = TEACHER_NAV.flatMap((group) => group.items);
const ROUTER = readFileSync(
  resolve(import.meta.dirname, "../../../src/app/router.tsx"),
  "utf8",
);

function comparable(permission: unknown): string {
  return JSON.stringify(
    Array.isArray(permission)
      ? [...permission].sort((a, b) => String(a).localeCompare(String(b)))
      : permission,
  );
}

function disagreements(items: readonly TeacherNavItem[]): string[] {
  return items.flatMap((item) => {
    const found = OPERATIONS.get(item.operation);
    if (found === undefined) return [`${item.id}: no operation ${item.operation}`];
    const problems: string[] = [];
    if (found.method !== "get" || !found.path.startsWith("/teacher/"))
      problems.push(`${item.id}: ${item.operation} is ${found.method} ${found.path}`);
    if (comparable(found.op["x-permission"]) !== comparable(item.requires))
      problems.push(
        `${item.id}: requires ${comparable(item.requires)}, ${item.operation} declares ${comparable(found.op["x-permission"])}`,
      );
    return problems;
  });
}

function teacherPaths(): Set<string> {
  const start = ROUTER.indexOf("const teacherTree: RouteObject = {");
  const tree = ROUTER.slice(start, ROUTER.indexOf("\n};", start));
  const paths = new Set(
    [...tree.matchAll(/path: "([^"]+)"/g)].map((match) =>
      match[1] === "teacher" ? "/teacher" : `/teacher/${match[1]}`,
    ),
  );
  if (!paths.has("/teacher") || !tree.includes("index: true"))
    throw new Error("no teacherTree with an index route in the router");
  return paths;
}

describe("the teacher sidebar against the contract", () => {
  it("opens, with each destination, a list the contract has, under the permission it declares", () => {
    expect(ITEMS).toHaveLength(8);
    expect(disagreements(ITEMS)).toEqual([]);
  });

  it("would be caught: Students behind the workspace alone", () => {
    const loosened = ITEMS.map((item) =>
      item.id === "students"
        ? { ...item, requires: "workspace.teacher" as const }
        : item,
    );
    expect(disagreements(loosened)).toEqual([
      'students: requires "workspace.teacher", listStudents declares "people.students.read"',
    ]);
  });

  it("would be caught: an any-of narrowed to one key, a key widened to a list, an operation that is not a teacher list", () => {
    const changed = ITEMS.map((item): TeacherNavItem => {
      if (item.id === "grading") return { ...item, requires: "teaching.grading" };
      if (item.id === "students")
        return { ...item, requires: ["people.students.read"] };
      if (item.id === "media") return { ...item, operation: "uploadMedia" };
      if (item.id === "tests") return { ...item, operation: "listEverything" };
      return item;
    });
    expect(disagreements(changed)).toEqual([
      'grading: requires "teaching.grading", listAttempts declares ["teaching.attempts.intervene","teaching.grading"]',
      'students: requires ["people.students.read"], listStudents declares "people.students.read"',
      "tests: no operation listEverything",
      "media: uploadMedia is post /teacher/media",
      'media: requires "workspace.teacher", uploadMedia declares "content.media.write"',
    ]);
  });

  it("reads an any-of as a set, whatever its order", () => {
    const reordered = ITEMS.map((item): TeacherNavItem =>
      item.id === "grading"
        ? { ...item, requires: ["teaching.attempts.intervene", "teaching.grading"] }
        : item,
    );
    expect(disagreements(reordered)).toEqual([]);
  });

  it("gives every destination its own address, each a path of the teacher tree", () => {
    const addresses = ITEMS.map((item) => item.to);
    expect(new Set(addresses).size).toBe(addresses.length);
    const paths = teacherPaths();
    expect(addresses.filter((address) => !paths.has(address))).toEqual([]);
  });
});
