import { holdSession } from "@/features/take-test/heldSession";
import { saveStrandedDraft } from "@/features/take-test/strandedDraft";
import { formatInTimeZone } from "date-fns-tz";
import { api, downloadFile } from "@/lib/api/client";
import { getDisplayTimeZone } from "@/lib/i18n/datetime";
import type { components } from "@/lib/api/schema";

export type Assignment = components["schemas"]["Assignment"];
export type AssignmentInput = components["schemas"]["AssignmentInput"];
export type AssignmentStatus = components["schemas"]["AssignmentStatus"];
export type ReviewPolicy = components["schemas"]["ReviewPolicy"];
export type IntegrityPolicy = components["schemas"]["IntegrityPolicy"];

export interface ListAssignmentsParams {
  status?: AssignmentStatus;
  /** Only assignments that target this class, or any of these; the facets follow them (G-12). */
  classId?: string | readonly string[];
  /** Test title or target class name, accent-insensitive; the facets follow it. */
  q?: string;
  page?: number;
  limit?: number;
}

export function listAssignments(
  params: ListAssignmentsParams = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  const classIds =
    typeof params.classId === "string" ? [params.classId] : params.classId;
  if (classIds?.length) query["classId"] = [...classIds];
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/teacher/assignments", signal ? { query, signal } : { query });
}

export function getAssignment(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/teacher/assignments/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createAssignment(body: AssignmentInput) {
  return api("post", "/teacher/assignments", { body });
}

export function updateAssignment(id: string, body: AssignmentInput) {
  return api("patch", "/teacher/assignments/{id}", { path: { id }, body });
}

export type AssignmentFacets = components["schemas"]["AssignmentStatusFacets"];

/** G-09's "Gia hạn cho tất cả": a later close, and the early close lifted. */
export function reopenAssignment(
  id: string,
  body: { closesAt: string; reason: string },
) {
  return api("post", "/teacher/assignments/{id}/reopen", { path: { id }, body });
}

export type StudentAssignmentCard = components["schemas"]["StudentAssignmentCard"];
export type StudentAssignmentDetail = components["schemas"]["StudentAssignmentDetail"];

export function listMyAssignments(signal?: AbortSignal) {
  return api("get", "/app/assignments", signal ? { signal } : {});
}

export function getMyAssignment(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/app/assignments/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

/**
 * continueAttempt first sends the answers a closed tab left in this browser,
 * through `saveStrandedDraft`, then resumes the named attempt and never starts
 * one: a 409 `ATTEMPT_CLOSED` means it has ended. It rejects without resuming
 * when those answers could not be sent. The tab then holds the session the
 * resume returned.
 */
export async function continueAttempt(assignmentId: string, attemptId: string) {
  await saveStrandedDraft(attemptId);
  const session = await api("post", "/app/assignments/{id}/attempts", {
    path: { id: assignmentId },
    body: { resume: attemptId },
  });
  holdSession(session.attempt.id, session.sessionId);
  return session;
}

/**
 * extendAssignment moves the assignment's close later by `minutes` for
 * everyone, and asks for the students to be told when `notify` is set. A
 * closed assignment answers `ASSIGNMENT_CLOSED`: it is reopened instead.
 */
export function extendAssignment(id: string, minutes: number, notify: boolean) {
  return api("post", "/teacher/assignments/{id}/extend", {
    path: { id },
    body: { minutes, notify },
  });
}

/** duplicateAssignment copies an assignment as a draft assigned to `classIds`. */
export function duplicateAssignment(id: string, classIds: readonly string[]) {
  return api("post", "/teacher/assignments/{id}/duplicate", {
    path: { id },
    body: { classIds: [...classIds] },
  });
}

/**
 * resultsFileName is the name a results export is saved under when the
 * server gives none: the assignment's title for one assignment, "results"
 * for several, then the day, as "unit-5-20260829.csv".
 */
export function resultsFileName(
  titles: readonly string[],
  now: Date,
  zone = getDisplayTimeZone(),
): string {
  const base =
    titles.length === 1 && titles[0] !== undefined
      ? titles[0]
          .normalize("NFD")
          .replaceAll(/\p{M}/gu, "")
          .replaceAll(/[đĐ]/g, "d")
          .toLowerCase()
          .replaceAll(/[^a-z0-9]+/g, "-")
          .replaceAll(/^-|-$/g, "")
      : "";
  return `${base || "results"}-${formatInTimeZone(now, zone, "yyyyMMdd")}.csv`;
}

const REVOKE_AFTER_MS = 40_000;

/**
 * exportResults downloads the results of up to 50 assignments as one CSV
 * file, under the name the server gives it or else `resultsFileName`, and
 * hands it to the browser to save through an object URL. The URL is revoked
 * 40 seconds later, not at once, because Firefox and some Safari versions
 * start the download after the click returns.
 */
export async function exportResults(
  assignments: readonly Pick<Assignment, "id" | "testTitle">[],
) {
  const file = await downloadFile("/teacher/assignments/results.csv", {
    ids: assignments.map((assignment) => assignment.id),
  });
  const href = URL.createObjectURL(file.blob);
  const link = document.createElement("a");
  link.href = href;
  link.download =
    file.filename ??
    resultsFileName(
      assignments.map((assignment) => assignment.testTitle),
      new Date(),
    );
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(href), REVOKE_AFTER_MS);
}

export function deleteAssignment(id: string) {
  return api("delete", "/teacher/assignments/{id}", { path: { id } });
}

export type ItemAnalysis = components["schemas"]["ItemAnalysis"];
export type StudentOverrideInput = components["schemas"]["StudentOverrideInput"];

/** getItemAnalysis is how often students got each question of the assignment's version wrong, hardest first. */
export function getItemAnalysis(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/teacher/assignments/{id}/item-analysis",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

/** setStudentOverrides gives the named students an override, or changes theirs, with the reason. */
export function setStudentOverrides(id: string, body: StudentOverrideInput) {
  return api("put", "/teacher/assignments/{id}/student-overrides", {
    path: { id },
    body,
  });
}
