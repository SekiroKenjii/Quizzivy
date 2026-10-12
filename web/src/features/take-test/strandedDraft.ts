import { ApiError, type ApiErrorCode } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import { saveAnswers } from "./api";
import { dropDraft, strandedDraft } from "./draft";

const refusals: ReadonlySet<string> = new Set<ApiErrorCode>([
  "SESSION_SUPERSEDED",
  "DEADLINE_PASSED",
  "ATTEMPT_CLOSED",
  "VALIDATION_FAILED",
]);

/**
 * saveStrandedDraft sends the answers a closed tab left in this browser for
 * the attempt, under the session that wrote them, and returns once the server
 * has saved them or refused them with one of the attempt's own codes; the
 * draft is then removed. It rejects, leaving the draft, on any other failure.
 */
export async function saveStrandedDraft(attemptId: string): Promise<void> {
  const studentId = useAuthStore.getState().user?.id;
  if (studentId === undefined) return;
  const draft = strandedDraft(attemptId, studentId);
  if (draft === null) return;
  try {
    await saveAnswers(attemptId, {
      sessionId: draft.sessionId,
      answers: draft.answers,
    });
  } catch (error) {
    if (!(error instanceof ApiError) || !refusals.has(error.code)) throw error;
  }
  dropDraft(attemptId, draft.raw);
}
