const prefix = "quizzivy.open-question.";

/**
 * readOpenQuestion returns the id of the question this tab last had open on
 * the attempt, or null when there is none or the storage cannot be read.
 */
export function readOpenQuestion(attemptId: string): string | null {
  try {
    return sessionStorage.getItem(prefix + attemptId);
  } catch {
    return null;
  }
}

/**
 * writeOpenQuestion remembers the open question's id for the tab's lifetime.
 * A storage that refuses the write is ignored.
 */
export function writeOpenQuestion(attemptId: string, questionId: string): void {
  try {
    sessionStorage.setItem(prefix + attemptId, questionId);
  } catch {
    return;
  }
}

/**
 * clearOpenQuestion forgets the attempt's open question. A storage that
 * refuses is ignored.
 */
export function clearOpenQuestion(attemptId: string): void {
  try {
    sessionStorage.removeItem(prefix + attemptId);
  } catch {
    return;
  }
}
