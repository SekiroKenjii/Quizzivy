import { describe, expect, it } from "vitest";
import { teacherHandleOf, type TeacherHandle } from "@/layouts/shell/handle";

describe("teacherHandleOf", () => {
  it("answers a handle whose crumb is an array, as it was given", () => {
    const handle = {
      crumb: [
        { key: "teacherShell.nav.tests", to: "/teacher/tests" },
        { key: "tests.detail" },
      ],
      width: 1080,
      sidebar: "collapsed",
    } satisfies TeacherHandle;
    expect(teacherHandleOf(handle)).toBe(handle);
  });

  it("answers a handle with an empty trail", () => {
    const handle = { crumb: [] };
    expect(teacherHandleOf(handle)).toBe(handle);
  });

  it.each([
    ["nothing", undefined],
    ["null", null],
    ["an empty handle", {}],
    ["a student detail handle", { detail: { titleKey: "nav.settings", back: "/app" } }],
    ["the take-test handle", { focus: true }],
    ["a crumb that is one object", { crumb: { key: "nav.tests" } }],
    ["a crumb that is a string", { crumb: "nav.tests" }],
    ["a string", "crumb"],
    ["an array", [{ key: "nav.tests" }]],
  ])("answers null for %s", (_what, handle) => {
    expect(teacherHandleOf(handle)).toBeNull();
  });
});
