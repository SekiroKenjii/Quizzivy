import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const ALLOWED_FILES = new Set([
  "app/adminPaths.ts",
  "app/legacyTeacherPath.ts",
  "app/LegacyTeacherRedirect.tsx",
  "lib/api/schema.d.ts",
]);
const ALLOWED_LITERALS = new Set([
  "/admin/ai/*",
  "/admin/ai/credits",
  "/admin/api-reference",
  "/admin/api-reference/*",
  "/admin/api-reference/openapi.json",
  "/admin/audit-log",
  "/admin/audit-log/export",
  "/admin/data-exports/*",
  "/admin/docs-session",
  "/admin/overview",
  "/admin/roles/*",
  "/admin/settings/*",
  "/admin/users/*",
  "/admin/users",
  "/admin/users/{id}",
  "/admin/settings/sign-in",
  "/admin/settings/logo",
  "/admin/settings/sign-in-impact",
  "/admin/roles",
  "/admin/roles/permissions",
  "/admin/roles/{id}",
]);
const LITERAL = /(["'`])(\/admin(?=[/"'`?#$]|$)[^"'`]*)/gi;
const SETTINGS_API_CALL = /\bapi\("(?:get|put)", "\/admin\/settings"/g;

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
    const routes = [...line.replaceAll(SETTINGS_API_CALL, "").matchAll(LITERAL)].filter(
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
    ['<Link to="/Admin/tests">', "another letter case, which the router also matches"],
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
    ['<Link to="/admin/settings/sign-in/extra">', "longer than the settings route"],
    ['<Link to="/admin/settings">', "the settings route without its section"],
    [
      '<Link to="/admin/audit">',
      "the audit screen, which a link reaches by ADMIN_PATHS",
    ],
    ['<Link to="/admin/api-reference/extra">', "longer than the reference's API path"],
    ['navigate("/admin/settings")', "the settings route, navigated to"],
    ['api("delete", "/admin/settings")', "a verb the settings API does not take"],
  ])("would be caught: %s (%s)", (line) => {
    expect(offenders("features/x/Planted.tsx", line)).toEqual([
      `features/x/Planted.tsx:1: ${line}`,
    ]);
  });

  it.each([
    ['api("post", "/admin/docs-session")', "the docs session"],
    ['api("delete", "/admin/users/{id}", { path: { id } })', "removing a user"],
    ['api("get", "/admin/ai/credits", { signal })', "the credits read"],
    ['api("get", "/admin/roles", { signal })', "the roles read"],
    ['api("put", "/admin/roles/permissions", { body })', "the matrix save"],
    ['api("patch", "/admin/roles/{id}", { path: { id }, body })', "an edit of a role"],
    ['api("get", "/admin/api-reference", { signal })', "the API reference summary"],
    ['downloadFile("/admin/api-reference/openapi.json")', "the contract download"],
    ['api("get", "/admin/audit-log", { query })', "the audit log read"],
    ['downloadFile("/admin/audit-log/export", query)', "the audit log export"],
    ['<Link to="/admin/settings/sign-in">', "the Admin console's sign-in settings"],
    ['api("get", "/admin/settings", { signal })', "reading the settings document"],
    ['api("put", "/admin/settings", { body: input })', "saving the settings document"],
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
