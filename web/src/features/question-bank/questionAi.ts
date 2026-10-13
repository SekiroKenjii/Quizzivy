import type { QuestionValues } from "./questionSchema";

/**
 * QuestionReady is the answer of `QuestionAi.prepare`: the id of a stored
 * question that holds what the editor shows, or the sentence that says why the
 * assistant cannot read it yet.
 */
export type QuestionReady = { id: string } | { refusal: string };

/**
 * QuestionAi is what a page gives its question editor so the editor offers
 * "Write with AI" and "Suggest wrong options". The operations read the stored
 * question, so `prepare` makes the stored question match the editor first (the
 * builder flushes its autosave) or refuses (the bank asks for a save). A page
 * that gives none draws neither.
 */
export interface QuestionAi {
  prepare: () => Promise<QuestionReady>;
}

function hasKey(values: QuestionValues): boolean {
  switch (values.type) {
    case "fill_blank":
      return values.blanks.some((blank) => blank.acceptedAnswers.length > 0);
    case "short_answer":
      return true;
    default:
      return values.options.some(
        (option) => option.isCorrect && option.text.trim() !== "",
      );
  }
}

/**
 * aiGuardOf names the deck's reason not to ask the assistant about a question
 * yet, as an i18n key: no prompt, or no correct answer. It is null when the
 * question can be asked about.
 */
export function aiGuardOf(values: QuestionValues): string | null {
  if (values.prompt.trim() === "") return "ai.guard.prompt";
  return hasKey(values) ? null : "ai.guard.key";
}

/**
 * aiReadsOf is the part of a question the assistant reads (its type, prompt,
 * written options, blanks' accepted answers and sample answer) as one string,
 * so two values compare equal exactly when the assistant would read them
 * alike. An empty option is left out: it cannot be stored, and the assistant
 * fills it.
 */
export function aiReadsOf(values: QuestionValues): string {
  return JSON.stringify([
    values.type,
    values.prompt,
    values.options
      .filter((option) => option.text.trim() !== "")
      .map((option) => [option.text, option.isCorrect]),
    values.blanks.map((blank) => blank.acceptedAnswers),
    values.sampleAnswer,
  ]);
}
