import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type AttemptSession = components["schemas"]["AttemptSession"];
export type StudentQuestion = components["schemas"]["StudentQuestion"];
export type Answer = components["schemas"]["Answer"];
export type Attempt = components["schemas"]["Attempt"];
export type IntegrityPolicy = components["schemas"]["IntegrityPolicy"];
export type IntegrityEventInput = components["schemas"]["IntegrityEventInput"];

export function startOrResumeAttempt(assignmentId: string, signal?: AbortSignal) {
  return api("post", "/app/assignments/{id}/attempts", {
    path: { id: assignmentId },
    ...(signal ? { signal } : {}),
  });
}

export function getAttempt(attemptId: string, signal?: AbortSignal) {
  return api("get", "/app/attempts/{id}", {
    path: { id: attemptId },
    ...(signal ? { signal } : {}),
  });
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

export function submitAttempt(
  attemptId: string,
  body: {
    sessionId?: string;
    reason?: "manual" | "timer_expired" | "auto_submit";
  } = {},
) {
  return api("post", "/app/attempts/{id}/submit", { path: { id: attemptId }, body });
}
