import { anchorSequence } from "@/features/integrity/buffer";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { heldSession, holdSession } from "./heldSession";

export type AttemptSession = components["schemas"]["AttemptSession"];
export type StudentQuestion = components["schemas"]["StudentQuestion"];
export type StudentSection = components["schemas"]["StudentSection"];
export type StudentGroup = components["schemas"]["StudentGroup"];
export type Answer = components["schemas"]["Answer"];
export type Attempt = components["schemas"]["Attempt"];
export type IntegrityPolicy = components["schemas"]["IntegrityPolicy"];
export type IntegrityEventInput = components["schemas"]["IntegrityEventInput"];

/**
 * startOrResumeAttempt starts the assignment's attempt, or resumes the live
 * one, and resolves to its session. The tab then holds that session: the
 * server gave it to this request, so every later read of the attempt names it.
 */
export async function startOrResumeAttempt(assignmentId: string, signal?: AbortSignal) {
  const session = await api("post", "/app/assignments/{id}/attempts", {
    path: { id: assignmentId },
    ...(signal ? { signal } : {}),
  });
  holdSession(session.attempt.id, session.sessionId);
  return session;
}

/**
 * getAttempt reads the take-test payload, naming the session this tab holds
 * for the attempt, or none when it holds none. The tab then holds the session
 * of the answer when the attempt is in progress and the answer does not say
 * `superseded`; a superseded answer, or one of an attempt that has ended,
 * changes nothing the tab holds. Every answer, superseded or not, anchors the
 * attempt's event sequence on the server's clock.
 */
export async function getAttempt(attemptId: string, signal?: AbortSignal) {
  const held = heldSession(attemptId);
  const session = await api("get", "/app/attempts/{id}", {
    path: { id: attemptId },
    query: held === null ? {} : { session: held },
    ...(signal ? { signal } : {}),
  });
  anchorSequence(session.attempt.id, session.attempt.startedAt, session.serverTime);
  if (session.attempt.status === "in_progress" && session.superseded !== true)
    holdSession(attemptId, session.sessionId);
  return session;
}

export function saveAnswers(
  attemptId: string,
  body: {
    sessionId: string;
    answers?: Record<string, Answer>;
    events?: IntegrityEventInput[];
  },
  signal?: AbortSignal,
) {
  return api("patch", "/app/attempts/{id}/answers", {
    path: { id: attemptId },
    body,
    ...(signal ? { signal } : {}),
  });
}

export function recordAudioPlay(attemptId: string, questionId: string) {
  return api("post", "/app/attempts/{id}/audio-play", {
    path: { id: attemptId },
    body: { questionId },
  });
}

export function recordGroupAudioPlay(
  attemptId: string,
  body: components["schemas"]["GroupAudioPlayInput"],
  signal?: AbortSignal,
) {
  return api("post", "/app/attempts/{id}/group-audio-play", {
    path: { id: attemptId },
    body,
    ...(signal ? { signal } : {}),
  });
}

export function submitAttempt(
  attemptId: string,
  body: {
    sessionId?: string;
    reason?: "manual" | "timer_expired" | "auto_submit";
  } = {},
) {
  return api("post", "/app/attempts/{id}/submit", { path: { id: attemptId }, body });
}
