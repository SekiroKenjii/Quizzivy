import { api } from "@/lib/api/client";
import type { components, paths } from "@/lib/api/schema";

export type MonitorRow = components["schemas"]["MonitorRow"];
export type MonitorState = MonitorRow["state"];
export type Monitor =
  paths["/admin/assignments/{id}/attempts"]["get"]["responses"][200]["content"]["application/json"];
export type AttemptReview =
  paths["/admin/attempts/{id}"]["get"]["responses"][200]["content"]["application/json"];
export type ReviewAnswer = AttemptReview["answers"][string];
export type AttemptEvents =
  paths["/admin/attempts/{id}/events"]["get"]["responses"][200]["content"]["application/json"];
export type IntegrityEvent = components["schemas"]["IntegrityEvent"];
export type IntegritySummary = components["schemas"]["IntegritySummary"];
export type AttemptListRow = components["schemas"]["AttemptListRow"];
export type AdminQuestion = components["schemas"]["AdminQuestion"];
export type AttemptScore = components["schemas"]["AttemptScore"];
export type Attempt = components["schemas"]["Attempt"];
export type AttemptStatus = components["schemas"]["AttemptStatus"];

export function getMonitor(assignmentId: string, signal?: AbortSignal) {
  return api("get", "/admin/assignments/{id}/attempts", {
    path: { id: assignmentId },
    ...(signal ? { signal } : {}),
  });
}

export interface ListAttemptsParams {
  status?: AttemptStatus;
  flagged?: boolean;
  pendingGrading?: boolean;
  page?: number;
  limit?: number;
}

export function listAttempts(params: ListAttemptsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.flagged !== undefined) query["flagged"] = params.flagged;
  if (params.pendingGrading !== undefined)
    query["pendingGrading"] = params.pendingGrading;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/attempts", signal ? { query, signal } : { query });
}

export function getAttemptForReview(id: string, signal?: AbortSignal) {
  return api("get", "/admin/attempts/{id}", {
    path: { id },
    ...(signal ? { signal } : {}),
  });
}

export type AnswersByQuestion =
  paths["/admin/assignments/{id}/answers"]["get"]["responses"][200]["content"]["application/json"];
export type QuestionAnswerRow = components["schemas"]["QuestionAnswerRow"];

/** G-04's read: one question across every handed-in paper of the assignment. */
export function listAnswersForQuestion(
  assignmentId: string,
  questionId: string,
  signal?: AbortSignal,
) {
  const params = { path: { id: assignmentId }, query: { questionId } };
  return api(
    "get",
    "/admin/assignments/{id}/answers",
    signal ? { ...params, signal } : params,
  );
}

export function getAttemptEvents(id: string, signal?: AbortSignal) {
  return api("get", "/admin/attempts/{id}/events", {
    path: { id },
    ...(signal ? { signal } : {}),
  });
}

export function extendAttempt(id: string, body: { minutes: number; reason: string }) {
  return api("post", "/admin/attempts/{id}/extend", { path: { id }, body });
}

/** G-05's mark, set or cleared by hand. */
export function flagAttempt(id: string, body: { flagged: boolean; reason?: string }) {
  return api("post", "/admin/attempts/{id}/flag", { path: { id }, body });
}

/** G-05's private note; null clears it. */
export function setAttemptNote(id: string, note: string | null) {
  return api("patch", "/admin/attempts/{id}/note", { path: { id }, body: { note } });
}

export function resetAttempt(id: string, body: { reason: string }) {
  return api("post", "/admin/attempts/{id}/reset", { path: { id }, body });
}

export function voidAttempt(id: string, body: { reason: string }) {
  return api("post", "/admin/attempts/{id}/void", { path: { id }, body });
}

export interface GradeItem {
  questionId: string;
  points: number;
  comment?: string | null;
}

export function gradeAttempt(id: string, items: GradeItem[]) {
  return api("post", "/admin/attempts/{id}/grade", { path: { id }, body: { items } });
}

export function finishGrading(id: string) {
  return api("post", "/admin/attempts/{id}/finish-grading", { path: { id } });
}

/** The monitor's "settled" states: the paper is out of the student's hands. */
export function isHandedIn(state: MonitorState): boolean {
  return state === "submitted" || state === "timed_out" || state === "graded";
}
