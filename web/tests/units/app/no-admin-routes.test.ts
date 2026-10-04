import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const ALLOWED_FILES = new Set([
  "app/legacyTeacherPath.ts",
  "app/LegacyTeacherRedirect.tsx",
  "lib/api/schema.d.ts",
]);
const ALLOWED_LITERALS = new Set(["/admin/docs-session", "/admin/users/{id}"]);
const LITERAL = /(["'`])(\/admin(?=[/"'`?#$]|$)[^"'`]*)/g;

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

function offenders(file: string, source: string): string[] {
  if (ALLOWED_FILES.has(file)) return [];
  return source.split("\n").flatMap((line, index) => {
    const routes = [...line.matchAll(LITERAL)].filter(
      (match) => !ALLOWED_LITERALS.has(match[2]!),
    );
    return routes.length > 0 ? [`${file}:${index + 1}: ${line.trim()}`] : [];
  });
}

describe("the teacher's routes live at /teacher", () => {
  it("has no /admin route literal in application code", () => {
    const found = sources(SRC).flatMap((path) =>
      offenders(relative(SRC, path).replaceAll("\\", "/"), readFileSync(path, "utf8")),
    );
    expect(found).toEqual([]);
  });

  it("reads every source file, the allowed ones included", () => {
    const files = sources(SRC).map((path) => relative(SRC, path).replaceAll("\\", "/"));
    expect(files.length).toBeGreaterThan(300);
    for (const file of ALLOWED_FILES) expect(files).toContain(file);
  });

  it("would be caught: a planted link names its file and its line", () => {
    const planted = [
      'import { Link } from "react-router";',
      "",
      "export function Planted() {",
      '  return <Link to="/admin/tests">Đề thi</Link>;',
      "}",
    ].join("\n");
    expect(offenders("features/tests/components/Planted.tsx", planted)).toEqual([
      'features/tests/components/Planted.tsx:4: return <Link to="/admin/tests">Đề thi</Link>;',
    ]);
  });

  it.each([
    ['<Link to="/admin">', "the bare prefix"],
    ["navigate('/admin/tests')", "single quotes"],
    ["navigate(`/admin/tests/${id}/edit`)", "a template"],
    ["navigate(`/admin${search}`)", "a template that goes on with a value"],
    ['href="/admin?tab=draft"', "a query"],
    ['href="/admin#versions"', "a hash"],
    ['{ path: "/admin/*", element: <Old /> }', "a route object"],
    ['if (pathname.startsWith("/admin/")) return "teacher";', "a prefix check"],
    ['api("delete", "/admin/users/{id}/sessions")', "a longer path than the API's"],
    ['api("get", `/admin/users/${id}`)', "the API's path, built by hand"],
    [
      'const a = "/admin/docs-session"; const b = "/admin/tests";',
      "one beside an API path",
    ],
    ["const to = `/admin", "a template left open on its line"],
  ])("would be caught: %s (%s)", (line) => {
    expect(offenders("features/x/Planted.tsx", line)).toEqual([
      `features/x/Planted.tsx:1: ${line}`,
    ]);
  });

  it.each([
    ['api("post", "/admin/docs-session")', "the docs session"],
    ['api("delete", "/admin/users/{id}", { path: { id } })', "removing a user"],
    ['<Link to="/teacher/tests">', "the new address"],
    ['<Link to="/administrator">', "a longer word"],
    ['<Link to="/admin-console">', "a longer segment"],
    ['<Link to="/app/admin">', "the word further along the path"],
    ['hasWorkspace(user, "admin")', "the workspace's name"],
    [
      '{ path: "admin/*", element: <LegacyTeacherRedirect /> }',
      "a relative route path",
    ],
  ])("lets through: %s (%s)", (line) => {
    expect(offenders("features/x/Planted.tsx", line)).toEqual([]);
  });

  it.each([
    "app/router.tsx",
    "app/pages/Planted.tsx",
    "app/legacyTeacherPath.test.ts",
    "lib/api/client.ts",
    "lib/api/schema.ts",
    "features/app/legacyTeacherPath.ts",
  ])("allows a file by its exact path, so %s is still read", (file) => {
    expect(offenders(file, 'const to = "/admin/tests";')).toEqual([
      `${file}:1: const to = "/admin/tests";`,
    ]);
  });

  it.each([...ALLOWED_FILES])("allows %s", (file) => {
    expect(offenders(file, 'const to = "/admin/tests";')).toEqual([]);
  });
});
