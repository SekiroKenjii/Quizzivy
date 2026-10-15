import type { PermissionKey } from "@/features/auth/permissions";

type HiddenKey =
  "scope.all" | "system.api_reference" | "system.data_export" | "system.leads";

/** MatrixKey is a permission the matrix draws a row for: every key but the four hidden ones. */
export type MatrixKey = Exclude<PermissionKey, HiddenKey>;

/** PermissionCopy is the i18n keys of a matrix row: its label, and a hint beneath it when it has one. */
export interface PermissionCopy {
  label: string;
  hint?: string;
}

/**
 * PERMISSION_COPY names every matrix row. It is a record over the generated
 * permission keys, so a key added to the contract and left out here fails
 * `pnpm typecheck`.
 */
export const PERMISSION_COPY: Readonly<Record<MatrixKey, PermissionCopy>> = {
  "content.tests.write": { label: "roles.perm.contentTestsWrite" },
  "content.tests.publish": {
    label: "roles.perm.contentTestsPublish",
    hint: "roles.hint.contentTestsPublish",
  },
  "content.questions.write": { label: "roles.perm.contentQuestionsWrite" },
  "content.media.write": { label: "roles.perm.contentMediaWrite" },
  "content.share": {
    label: "roles.perm.contentShare",
    hint: "roles.hint.contentShare",
  },
  "ai.use": { label: "roles.perm.aiUse" },
  "teaching.classes.write": { label: "roles.perm.teachingClassesWrite" },
  "teaching.assignments.write": { label: "roles.perm.teachingAssignmentsWrite" },
  "teaching.grading": { label: "roles.perm.teachingGrading" },
  "teaching.attempts.intervene": {
    label: "roles.perm.teachingAttemptsIntervene",
    hint: "roles.hint.audited",
  },
  "teaching.attendance": { label: "roles.perm.teachingAttendance" },
  "people.students.read": { label: "roles.perm.peopleStudentsRead" },
  "people.students.create": { label: "roles.perm.peopleStudentsCreate" },
  "people.students.reset_password": {
    label: "roles.perm.peopleStudentsResetPassword",
    hint: "roles.hint.audited",
  },
  "people.users.manage": { label: "roles.perm.peopleUsersManage" },
  "people.roles.manage": { label: "roles.perm.peopleRolesManage" },
  "system.audit.read": { label: "roles.perm.systemAuditRead" },
  "system.settings.write": { label: "roles.perm.systemSettingsWrite" },
  "learning.take_tests": { label: "roles.perm.learningTakeTests" },
  "ai.credits.manage": { label: "roles.perm.aiCreditsManage" },
};

/** copyOf returns the copy of a catalogue key, or null for a key this build has no row for. */
export function copyOf(key: PermissionKey): PermissionCopy | null {
  return (
    (PERMISSION_COPY as Readonly<Partial<Record<PermissionKey, PermissionCopy>>>)[
      key
    ] ?? null
  );
}
