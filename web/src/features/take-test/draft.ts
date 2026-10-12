import { z } from "zod";
import type { Answer } from "./api";

const prefix = "quizzivy.answer-draft.";
const schema = z.object({
  studentId: z.string(),
  sessionId: z.string(),
  deadlineAt: z.number(),
  answers: z.record(
    z.string(),
    z.discriminatedUnion("type", [
      z.object({ type: z.literal("choice"), optionIds: z.array(z.string()) }),
      z.object({ type: z.literal("true_false"), value: z.boolean() }),
      z.object({ type: z.literal("text"), value: z.string() }),
      z.object({
        type: z.literal("fill_blank"),
        values: z.record(z.string(), z.string()),
      }),
    ]),
  ),
});

/** readDraft returns only this student's unconfirmed answers before the attempt deadline. */
export function readDraft(
  attemptId: string,
  studentId: string,
  sessionId: string,
  serverTime: number,
): Record<string, Answer> {
  try {
    const raw = localStorage.getItem(prefix + attemptId);
    if (raw === null) return {};
    const parsed = schema.safeParse(JSON.parse(raw));
    if (!parsed.success || parsed.data.deadlineAt <= serverTime) {
      clearDraft(attemptId);
      return {};
    }
    return parsed.data.studentId === studentId && parsed.data.sessionId === sessionId
      ? parsed.data.answers
      : {};
  } catch {
    return {};
  }
}

/** writeDraft persists unconfirmed answers without tokens, including during a network outage. */
export function writeDraft(
  attemptId: string,
  studentId: string,
  sessionId: string,
  deadlineAt: number,
  answers: Record<string, Answer>,
): void {
  try {
    if (Object.keys(answers).length === 0) clearDraft(attemptId);
    else
      localStorage.setItem(
        prefix + attemptId,
        JSON.stringify({ studentId, sessionId, deadlineAt, answers }),
      );
  } catch {
    return;
  }
}

/** clearDraft removes local unconfirmed answers after saving or closing an attempt. */
export function clearDraft(attemptId: string): void {
  try {
    localStorage.removeItem(prefix + attemptId);
  } catch {
    return;
  }
}

/**
 * strandedDraft returns the draft this browser holds for the attempt, whatever
 * session wrote it, when it is this student's and holds at least one answer,
 * and null otherwise. It changes nothing: a draft that does not parse, that is
 * another student's or that is past its own deadline is neither rewritten nor
 * removed.
 */
export function strandedDraft(
  attemptId: string,
  studentId: string,
): { raw: string; sessionId: string; answers: Record<string, Answer> } | null {
  try {
    const raw = localStorage.getItem(prefix + attemptId);
    if (raw === null) return null;
    const parsed = schema.safeParse(JSON.parse(raw));
    if (
      !parsed.success ||
      parsed.data.studentId !== studentId ||
      Object.keys(parsed.data.answers).length === 0
    )
      return null;
    return { raw, sessionId: parsed.data.sessionId, answers: parsed.data.answers };
  } catch {
    return null;
  }
}

/**
 * dropDraft removes the draft that was read as `raw`, and leaves one a live
 * tab has rewritten since.
 */
export function dropDraft(attemptId: string, raw: string): void {
  try {
    if (localStorage.getItem(prefix + attemptId) === raw)
      localStorage.removeItem(prefix + attemptId);
  } catch {
    return;
  }
}

/** clearAnswerDrafts removes student answers from a shared browser on sign-out. */
export function clearAnswerDrafts(): void {
  try {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith(prefix)) localStorage.removeItem(key);
    }
  } catch {
    return;
  }
}
