import { describe, expect, it } from "vitest";
import { legacyTeacherPath } from "@/app/legacyTeacherPath";

describe("legacyTeacherPath", () => {
  it.each([
    ["/admin", "/teacher"],
    ["/admin/", "/teacher/"],
    ["/admin/tests/abc/edit", "/teacher/tests/abc/edit"],
    ["/admin/settings/security", "/teacher/settings/security"],
    ["/admin/admin", "/teacher/admin"],
    ["/Admin", "/teacher"],
    ["/ADMIN/Tests/abc", "/teacher/Tests/abc"],
  ])("maps %s to %s", (from, to) => {
    expect(legacyTeacherPath(from)).toBe(to);
  });

  it.each([
    "/administrator",
    "/administrator/tests",
    "/admin-old",
    "/admin.html",
    "/app/admin",
    "/teacher/admin",
    "/teacher",
    "admin",
    "admin/tests",
    "/",
    "",
  ])("gives null for %j", (path) => {
    expect(legacyTeacherPath(path)).toBeNull();
  });
});
