import { gapBindingsMatch } from "@/components/shared/content/gaps";
import { isQuestionPromptContent } from "@/components/shared/content/questionContent";
import type { TFunction } from "i18next";
import type { AdminQuestion } from "@/features/question-bank/api";
import {
  comparePlaceholders,
  hasMismatch,
} from "@/features/question-bank/placeholders";

/** publishProblem supplies immediate outline feedback for the server's publish rules. */
export function publishProblem(question: AdminQuestion, t: TFunction): string | null {
  if (question.points <= 0) return t("questionEditor.pointsError");
  if (
    ["single_choice", "multiple_choice", "true_false"].includes(question.type) &&
    !question.options?.some((option) => option.isCorrect)
  )
    return t("builder.correctOptionRequired");
  if (question.type === "fill_blank") {
    const blanks = question.blanks ?? [];
    if (blanks.some((blank) => blank.acceptedAnswers.length === 0))
      return t("builder.blankAnswerRequired");
    if (
      question.promptContent != null
        ? !isQuestionPromptContent(question.promptContent) ||
          !gapBindingsMatch(question.promptContent, blanks)
        : hasMismatch(
            comparePlaceholders(
              question.prompt,
              blanks.map((blank) => blank.ordinal),
            ),
          )
    )
      return t("builder.blankMismatch");
  }
  if (question.audio && question.media?.kind !== "audio")
    return t("builder.audioRequired");
  return null;
}
