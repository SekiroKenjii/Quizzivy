import { describe, expect, it } from "vitest";
import {
  changesOf,
  lockOf,
  NO_DRAFT,
  revisionsOf,
  takingNow,
  toggled,
  type Caller,
} from "@/features/roles/cells";
import type { PermissionKey } from "@/features/auth/permissions";
import {
  ADMIN_ID,
  CATALOGUE,
  HEAD_ID,
  MATRIX_KEYS,
  ROLES,
  TEACHER_ID,
  admin,
  assistant,
  head,
  student,
  teacher,
} from "@tests/support/roles";

const adminCaller: Caller = { roleId: ADMIN_ID, holds: new Set(MATRIX_KEYS) };
const headCaller: Caller = {
  roleId: HEAD_ID,
  holds: new Set<PermissionKey>([...head.permissions, "people.roles.manage"]),
};

describe("lockOf", () => {
  it("locks every Admin cell but Take tests, whoever asks", () => {
    expect(lockOf(admin, "content.tests.write", adminCaller)).toBe("admin");
    expect(lockOf(admin, "system.settings.write", headCaller)).toBe("admin");
  });

  it("lets an Admin change the Admin role's own Take tests cell", () => {
    expect(lockOf(admin, "learning.take_tests", adminCaller)).toBeNull();
  });

  it("keeps Take tests on the Student role", () => {
    expect(lockOf(student, "learning.take_tests", adminCaller)).toBe("student");
    expect(lockOf(student, "content.tests.write", adminCaller)).toBeNull();
  });

  it("locks the whole of the caller's own role, the Admin's Take tests apart", () => {
    expect(lockOf(head, "content.tests.write", headCaller)).toBe("self");
    expect(lockOf(head, "learning.take_tests", headCaller)).toBe("self");
    expect(lockOf(admin, "learning.take_tests", adminCaller)).toBeNull();
  });

  it("applies the subset rule to a role the caller cannot act on", () => {
    const beyond = { ...teacher, editable: false };
    expect(lockOf(beyond, "content.tests.write", adminCaller)).toBe("subset");
  });

  it("applies the subset rule to a key the caller does not hold, Take tests apart", () => {
    expect(lockOf(assistant, "system.audit.read", headCaller)).toBe("subset");
    expect(lockOf(assistant, "content.tests.write", headCaller)).toBeNull();
    expect(lockOf(assistant, "learning.take_tests", headCaller)).toBeNull();
  });

  it("asks the checks in the server's order: Admin before Student before self", () => {
    const own: Caller = { roleId: ADMIN_ID, holds: new Set() };
    expect(lockOf(admin, "content.tests.write", own)).toBe("admin");
  });
});

describe("the draft", () => {
  it("turns a cell the other way, and back, leaving no trace", () => {
    const on = toggled(NO_DRAFT, teacher, "content.tests.write");
    expect(on.get(`${TEACHER_ID}|content.tests.write`)).toBe(false);
    const back = toggled(on, teacher, "content.tests.write");
    expect(back.size).toBe(0);
  });

  it("lists the cells that differ, in role then catalogue order", () => {
    let draft = toggled(NO_DRAFT, teacher, "system.audit.read");
    draft = toggled(draft, admin, "learning.take_tests");
    draft = toggled(draft, teacher, "content.tests.write");
    expect(changesOf(draft, ROLES, CATALOGUE)).toEqual([
      { roleId: ADMIN_ID, key: "learning.take_tests", granted: true },
      { roleId: TEACHER_ID, key: "content.tests.write", granted: false },
      { roleId: TEACHER_ID, key: "system.audit.read", granted: true },
    ]);
  });

  it("drops a cell that the server now holds as the draft wants it", () => {
    const draft = toggled(NO_DRAFT, teacher, "system.audit.read");
    const reloaded = {
      ...teacher,
      permissions: [...teacher.permissions, "system.audit.read" as const],
    };
    expect(changesOf(draft, [reloaded], CATALOGUE)).toEqual([]);
  });

  it("names the revision of each role a change touches, once", () => {
    const changes = changesOf(
      toggled(toggled(NO_DRAFT, teacher, "ai.use"), teacher, "content.share"),
      ROLES,
      CATALOGUE,
    );
    expect(revisionsOf(changes, ROLES)).toEqual([
      { roleId: TEACHER_ID, revision: teacher.revision },
    ]);
  });

  it("counts the people mid-test only for a role that loses Take tests", () => {
    const losing = changesOf(
      toggled(NO_DRAFT, head, "learning.take_tests"),
      ROLES,
      CATALOGUE,
    );
    expect(takingNow(losing, ROLES)).toBe(head.inProgressAttempts);
    const gaining = changesOf(
      toggled(NO_DRAFT, admin, "learning.take_tests"),
      ROLES,
      CATALOGUE,
    );
    expect(takingNow(gaining, ROLES)).toBe(0);
  });
});
