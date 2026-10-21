import { http } from "msw";
import type { components } from "@/lib/api/schema";
import { contractJson } from "./contractResponse";
import { server } from "./server";

type RoleSummary = components["schemas"]["RoleSummary"];
type RoleList = components["schemas"]["RoleList"];
type Entry = components["schemas"]["PermissionCatalogueEntry"];
type Key = components["schemas"]["PermissionKey"];

export const BASE = "http://localhost:8080";

export const ADMIN_ID = "019535d8-0000-7000-8000-000000000001";
export const TEACHER_ID = "019535d8-0000-7000-8000-000000000002";
export const ASSISTANT_ID = "019535d8-0000-7000-8000-000000000003";
export const STUDENT_ID = "019535d8-0000-7000-8000-000000000004";
export const HEAD_ID = "019535d8-0000-7000-8000-0000000000a1";

const GROUPS: readonly [Entry["group"], readonly Key[]][] = [
  [
    "content",
    [
      "content.tests.write",
      "content.tests.publish",
      "content.questions.write",
      "content.media.write",
      "content.share",
      "ai.use",
    ],
  ],
  [
    "teaching",
    [
      "teaching.classes.write",
      "teaching.assignments.write",
      "teaching.grading",
      "teaching.attempts.intervene",
      "teaching.attendance",
    ],
  ],
  [
    "people",
    [
      "people.students.read",
      "people.students.create",
      "people.students.reset_password",
      "people.users.manage",
      "people.roles.manage",
    ],
  ],
  [
    "system",
    [
      "system.audit.read",
      "system.settings.write",
      "learning.take_tests",
      "ai.credits.manage",
    ],
  ],
];

export const CATALOGUE: Entry[] = GROUPS.flatMap(([group, keys]) =>
  keys.map((key) => ({ key, group, adminLocked: key !== "learning.take_tests" })),
);

export const MATRIX_KEYS: Key[] = CATALOGUE.map((entry) => entry.key);

const TEACHER_KEYS: Key[] = [
  "content.tests.write",
  "content.tests.publish",
  "content.questions.write",
  "content.media.write",
  "content.share",
  "ai.use",
  "teaching.classes.write",
  "teaching.assignments.write",
  "teaching.grading",
  "teaching.attempts.intervene",
  "teaching.attendance",
  "people.students.read",
  "people.students.create",
  "people.students.reset_password",
];

const base = {
  icon: "key-round",
  color: "gray",
  revision: 3,
  editable: true,
  inProgressAttempts: 0,
} as const;

export const admin: RoleSummary = {
  ...base,
  id: ADMIN_ID,
  builtinKey: "admin",
  name: "Quản trị viên",
  description: "Điều hành trung tâm. Quản lý người dùng và cài đặt.",
  icon: "shield",
  color: "dark",
  custom: false,
  userCount: 3,
  permissions: MATRIX_KEYS.filter((key) => key !== "learning.take_tests"),
};

export const teacher: RoleSummary = {
  ...base,
  id: TEACHER_ID,
  builtinKey: "teacher",
  name: "Giáo viên",
  description: "Soạn đề, quản lý lớp, chấm bài.",
  icon: "graduation-cap",
  color: "lime",
  custom: false,
  userCount: 9,
  permissions: TEACHER_KEYS,
};

export const assistant: RoleSummary = {
  ...base,
  id: ASSISTANT_ID,
  builtinKey: "assistant",
  name: "Trợ giảng",
  description: "Hỗ trợ giáo viên chấm bài và điểm danh.",
  icon: "hand-helping",
  color: "blue",
  custom: false,
  userCount: 0,
  permissions: [
    "content.tests.write",
    "content.questions.write",
    "teaching.assignments.write",
    "teaching.grading",
    "teaching.attendance",
    "people.students.read",
  ],
};

export const student: RoleSummary = {
  ...base,
  id: STUDENT_ID,
  builtinKey: "student",
  name: "Học viên",
  description: "Làm bài và xem kết quả của mình.",
  icon: "user",
  color: "gray",
  custom: false,
  userCount: 300,
  permissions: ["learning.take_tests"],
};

export const head: RoleSummary = {
  ...base,
  id: HEAD_ID,
  builtinKey: null,
  name: "Tổ trưởng bộ môn",
  description: "Dẫn dắt nhóm IELTS",
  custom: true,
  userCount: 2,
  inProgressAttempts: 2,
  permissions: [...TEACHER_KEYS.slice(0, 4), "learning.take_tests"],
};

export const ROLES: RoleSummary[] = [admin, teacher, assistant, student, head];

export function listOf(roles: RoleSummary[] = ROLES): RoleList {
  return { roles, catalogue: CATALOGUE };
}

/** serveRoles answers `listRoles`; the returned list is what a read gets, replaceable between reads. */
export function serveRoles(initial: RoleList = listOf()) {
  const seen = { reads: 0, list: initial };
  server.use(
    http.get(`${BASE}/admin/roles`, () => {
      seen.reads += 1;
      return contractJson("/admin/roles", "get", 200, seen.list);
    }),
  );
  return seen;
}
