import type { components } from "@/lib/api/schema";

/**
 * Shared fixtures, typed against the generated contract. The types stop a
 * fixture inventing a field; `contractJson` stops it from being the wrong
 * shape at runtime. Vietnamese names throughout, since that is the product.
 */

export const studentUser: components["schemas"]["CurrentUser"] = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
  email: "hocvien@example.com",
  fullName: "Nguyễn Văn An",
  role: "student",
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
  permissions: ["learning.take_tests"],
  workspaces: ["app"],
};

export const adminUser: components["schemas"]["CurrentUser"] = {
  ...studentUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17f9f",
  email: "thuong@example.com",
  fullName: "Thuong",
  role: "admin",
  permissions: [
    "content.tests.write",
    "content.tests.publish",
    "content.questions.write",
    "content.media.write",
    "content.share",
    "teaching.classes.write",
    "teaching.assignments.write",
    "teaching.grading",
    "teaching.attempts.intervene",
    "teaching.attendance",
    "people.students.read",
    "people.students.create",
    "people.students.reset_password",
    "people.users.manage",
    "people.roles.manage",
    "system.audit.read",
    "system.settings.write",
    "scope.all",
    "system.api_reference",
    "system.data_export",
    "system.leads",
  ],
  workspaces: ["teacher", "admin"],
};

export const teacherUser: components["schemas"]["CurrentUser"] = {
  ...studentUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17fa0",
  email: "giaovien@example.com",
  fullName: "Trần Thị Bình",
  role: "admin",
  permissions: [
    "content.tests.write",
    "content.tests.publish",
    "content.questions.write",
    "content.media.write",
    "content.share",
    "teaching.classes.write",
    "teaching.assignments.write",
    "teaching.grading",
    "teaching.attempts.intervene",
    "teaching.attendance",
    "people.students.read",
    "people.students.create",
    "people.students.reset_password",
  ],
  workspaces: ["teacher"],
};

export const assistantUser: components["schemas"]["CurrentUser"] = {
  ...studentUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17fa1",
  email: "trogiang@example.com",
  fullName: "Lê Văn Cường",
  role: "admin",
  permissions: [
    "content.tests.write",
    "content.questions.write",
    "teaching.assignments.write",
    "teaching.grading",
    "teaching.attendance",
    "people.students.read",
  ],
  workspaces: ["teacher"],
};

export const myClass: components["schemas"]["MyClass"] = {
  id: "019535da-0000-7000-8000-000000000001",
  name: "Tiếng Anh giao tiếp - Lớp A",
  description: null,
  teacherName: "Cô Thương",
  joinedAt: "2026-06-12T01:00:00Z",
};

export const sampleClass: components["schemas"]["Class"] = {
  id: "019535da-0000-7000-8000-000000000001",
  name: "Tiếng Anh giao tiếp - Lớp A",
  description: null,
  studentCount: 12,
  openAssignmentCount: 0,
  archivedAt: null,
  selfJoinEnabled: true,
  joinCode: null,
  createdAt: "2026-01-01T00:00:00Z",
};
