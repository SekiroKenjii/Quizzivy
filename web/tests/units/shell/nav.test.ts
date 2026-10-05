import { describe, expect, it } from "vitest";
import { activeNavId, navFor, TEACHER_NAV } from "@/layouts/teacherNav";
import type { PermissionKey, Workspace } from "@/features/auth/permissions";
import { adminUser, assistantUser, teacherUser } from "@tests/support/fixtures";

const WHOLE = [
  ["home", ["dashboard"]],
  ["teaching", ["assignments", "grading", "classes", "students"]],
  ["content", ["tests", "bank", "media"]],
];

function role(permissions: PermissionKey[], workspaces: Workspace[] = ["teacher"]) {
  return { permissions, workspaces };
}

function shown(user: Parameters<typeof navFor>[0]) {
  return navFor(user).map((group) => [group.id, group.items.map((item) => item.id)]);
}

describe("the teacher sidebar's destinations", () => {
  it("are the deck's eight for R4, in its order, under its three groups", () => {
    expect(
      TEACHER_NAV.map((group) => [
        group.label,
        group.items.map((item) => [item.label, item.to]),
      ]),
    ).toEqual([
      [undefined, [["teacherShell.nav.dashboard", "/teacher"]]],
      [
        "teacherShell.group.teaching",
        [
          ["teacherShell.nav.assignments", "/teacher/assignments"],
          ["teacherShell.nav.grading", "/teacher/grading"],
          ["teacherShell.nav.classes", "/teacher/classes"],
          ["teacherShell.nav.students", "/teacher/students"],
        ],
      ],
      [
        "teacherShell.group.content",
        [
          ["teacherShell.nav.tests", "/teacher/tests"],
          ["teacherShell.nav.questionBank", "/teacher/question-bank"],
          ["teacherShell.nav.media", "/teacher/media"],
        ],
      ],
    ]);
  });

  it("count live assignments quietly and papers to grade in the accent, and nothing else", () => {
    const counted = TEACHER_NAV.flatMap((group) => group.items)
      .filter((item) => item.count !== undefined)
      .map((item) => [item.id, item.count]);
    expect(counted).toEqual([
      ["assignments", { figure: "liveAssignments", tone: "muted" }],
      ["grading", { figure: "toGrade", tone: "urgent" }],
    ]);
  });
});

describe("the sidebar a user sees", () => {
  it.each([
    ["a teacher", teacherUser],
    ["the admin", adminUser],
    ["an assistant", assistantUser],
  ])("is whole for %s", (_who, user) => {
    expect(shown(user)).toEqual(WHOLE);
  });

  it("has no Students for a role without people.students.read", () => {
    expect(shown(role(["teaching.grading", "content.tests.write"]))).toEqual([
      ["home", ["dashboard"]],
      ["teaching", ["assignments", "grading", "classes"]],
      ["content", ["tests", "bank", "media"]],
    ]);
  });

  it("has no Grading for a role with neither grading key", () => {
    expect(shown(role(["people.students.read", "teaching.assignments.write"]))).toEqual(
      [
        ["home", ["dashboard"]],
        ["teaching", ["assignments", "classes", "students"]],
        ["content", ["tests", "bank", "media"]],
      ],
    );
  });

  it.each([["teaching.attempts.intervene"], ["teaching.grading"]] as const)(
    "has Grading for a role holding %s alone",
    (key) => {
      expect(shown(role([key]))[1]).toEqual([
        "teaching",
        ["assignments", "grading", "classes"],
      ]);
    },
  );

  it("is the six workspace destinations, in order, for a role with no key at all", () => {
    expect(shown(role([]))).toEqual([
      ["home", ["dashboard"]],
      ["teaching", ["assignments", "classes"]],
      ["content", ["tests", "bank", "media"]],
    ]);
  });

  it("drops a group left with no destination", () => {
    expect(
      shown(role(["people.students.read", "teaching.grading"], ["admin"])),
    ).toEqual([["teaching", ["grading", "students"]]]);
    expect(shown(role([], ["app"]))).toEqual([]);
    expect(shown(null)).toEqual([]);
  });
});

describe("the destination a path lights", () => {
  it.each([
    ["/teacher", "dashboard"],
    ["/teacher/tests", "tests"],
    ["/teacher/tests/018f-a1", "tests"],
    ["/teacher/tests/018f-a1/edit", "tests"],
    ["/teacher/imports", "tests"],
    ["/teacher/imports/", "tests"],
    ["/teacher/imports/new", "tests"],
    ["/teacher/imports/018f-b1", "tests"],
    ["/teacher/imports/018f-b1/review", "tests"],
    ["/teacher/question-bank", "bank"],
    ["/teacher/question-bank/groups", "bank"],
    ["/teacher/question-bank/groups/018f-c1", "bank"],
    ["/teacher/question-bank/new", "bank"],
    ["/teacher/question-bank/018f-d1", "bank"],
    ["/teacher/media", "media"],
    ["/teacher/assignments", "assignments"],
    ["/teacher/assignments/new", "assignments"],
    ["/teacher/assignments/018f-e1", "assignments"],
    ["/teacher/assignments/018f-e1/edit", "assignments"],
    ["/teacher/assignments/018f-e1/attempts", "assignments"],
    ["/teacher/attempts/018f-f1", "assignments"],
    ["/teacher/grading", "grading"],
    ["/teacher/students", "students"],
    ["/teacher/classes", "classes"],
    ["/teacher/classes/018f-g1", "classes"],
    ["/teacher/settings", null],
  ])("%s lights %s", (path, id) => {
    expect(activeNavId(path)).toBe(id);
  });

  it("lights Tests for an import page that does not exist yet", () => {
    expect(activeNavId("/teacher/imports/018f-b1/confirm")).toBe("tests");
  });

  it("lights nothing for a section of Settings", () => {
    expect(activeNavId("/teacher/settings/security")).toBeNull();
  });

  it.each([
    "/teacher/testsuite",
    "/teacher/importsx",
    "/teacher/attemptsx/1",
    "/teacher/gradings",
    "/teachers",
    "/app",
    "/",
  ])("lights nothing for %s, which only begins like a destination", (path) => {
    expect(activeNavId(path)).toBeNull();
  });

  it("reads a trailing slash and another letter case as the router does", () => {
    expect(activeNavId("/teacher/")).toBe("dashboard");
    expect(activeNavId("/teacher/classes/")).toBe("classes");
    expect(activeNavId("/Teacher/Question-Bank/New")).toBe("bank");
  });
});
