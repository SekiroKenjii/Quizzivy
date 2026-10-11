/** ModuleKey names a part of the student app that a later release builds. */
export type ModuleKey = "notifications" | "messages" | "schedule" | "grades" | "learn";

/**
 * modules says which of those parts have shipped. The student shell and its
 * pages show a destination, a button or a setting only when its module is on,
 * so nothing links to a screen that does not exist; the teacher shells' bell
 * follows `notifications` too. A release turns its flag
 * on in the task that ships the screen: notifications in R4, messages in R7,
 * schedule in R8, grades in R9 and learn in R10.
 */
export const modules: Record<ModuleKey, boolean> = {
  notifications: true,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
};
