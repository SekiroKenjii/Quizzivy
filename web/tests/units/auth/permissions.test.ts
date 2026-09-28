import { afterEach, describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { destinationAfterSignIn, homePathFor } from "@/features/auth/home";
import {
  can,
  hasWorkspace,
  learnsOnly,
  useCan,
  useWorkspace,
} from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import {
  adminUser,
  assistantUser,
  studentUser,
  teacherUser,
} from "@tests/support/fixtures";

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("can", () => {
  it("answers from the role's permissions", () => {
    expect(can(adminUser, "scope.all")).toBe(true);
    expect(can(adminUser, "learning.take_tests")).toBe(false);
    expect(can(teacherUser, "content.tests.publish")).toBe(true);
    expect(can(teacherUser, "people.users.manage")).toBe(false);
    expect(can(assistantUser, "teaching.grading")).toBe(true);
    expect(can(assistantUser, "content.tests.publish")).toBe(false);
    expect(can(studentUser, "learning.take_tests")).toBe(true);
    expect(can(studentUser, "content.tests.write")).toBe(false);
  });

  it("refuses everything to no one", () => {
    expect(can(null, "learning.take_tests")).toBe(false);
    expect(can(undefined, "content.tests.write")).toBe(false);
  });

  it("reads each user object's own permissions", () => {
    const narrowed = { ...teacherUser, permissions: ["content.tests.write" as const] };
    expect(can(teacherUser, "teaching.grading")).toBe(true);
    expect(can(narrowed, "teaching.grading")).toBe(false);
    expect(can(teacherUser, "teaching.grading")).toBe(true);
  });
});

describe("workspaces", () => {
  it("answers from the user's workspaces", () => {
    expect(hasWorkspace(adminUser, "admin")).toBe(true);
    expect(hasWorkspace(adminUser, "app")).toBe(false);
    expect(hasWorkspace(teacherUser, "teacher")).toBe(true);
    expect(hasWorkspace(teacherUser, "admin")).toBe(false);
    expect(hasWorkspace(studentUser, "app")).toBe(true);
    expect(hasWorkspace(null, "app")).toBe(false);
  });

  it("names only a user whose one workspace is the student app as a learner", () => {
    expect(learnsOnly(studentUser)).toBe(true);
    expect(learnsOnly(teacherUser)).toBe(false);
    expect(learnsOnly({ ...adminUser, workspaces: ["teacher", "admin", "app"] })).toBe(
      false,
    );
    expect(learnsOnly({ ...studentUser, workspaces: [] })).toBe(false);
    expect(learnsOnly(null)).toBe(false);
  });
});

describe("home", () => {
  it("sends the teacher and admin workspaces to /admin and everyone else to /app", () => {
    expect(homePathFor(adminUser)).toBe("/admin");
    expect(homePathFor(teacherUser)).toBe("/admin");
    expect(homePathFor(assistantUser)).toBe("/admin");
    expect(homePathFor({ ...adminUser, workspaces: ["admin"] })).toBe("/admin");
    expect(homePathFor(studentUser)).toBe("/app");
    expect(homePathFor({ ...studentUser, workspaces: [] })).toBe("/app");
    expect(homePathFor(null)).toBe("/app");
  });

  it("prefers a same-site next over the home", () => {
    expect(destinationAfterSignIn("/admin/tests", studentUser)).toBe("/admin/tests");
    expect(destinationAfterSignIn("//evil.example", teacherUser)).toBe("/admin");
    expect(destinationAfterSignIn(null, studentUser)).toBe("/app");
  });
});

describe("the hooks", () => {
  it("follow the signed-in user", () => {
    const { result } = renderHook(() => ({
      grade: useCan("teaching.grading"),
      teacher: useWorkspace("teacher"),
    }));
    expect(result.current).toEqual({ grade: false, teacher: false });

    act(() => useAuthStore.getState().setSession("t", assistantUser));
    expect(result.current).toEqual({ grade: true, teacher: true });

    act(() => useAuthStore.getState().setUser(studentUser));
    expect(result.current).toEqual({ grade: false, teacher: false });
  });

  it("do not re-render when the user changes but the answer does not", () => {
    useAuthStore.getState().setSession("t", teacherUser);
    let renders = 0;
    renderHook(() => {
      renders += 1;
      return [useCan("teaching.grading"), useWorkspace("teacher")];
    });
    const before = renders;

    act(() =>
      useAuthStore
        .getState()
        .setUser({ ...structuredClone(teacherUser), fullName: "Trần Bình" }),
    );
    expect(renders).toBe(before);
  });
});
