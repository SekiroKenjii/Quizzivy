import { gapBindingsMatch } from "@/components/shared/content/gaps";
import { isQuestionPromptContent } from "@/components/shared/content/questionContent";
import type { TFunction } from "i18next";
import type { AdminQuestion } from "@/features/question-bank/api";
import {
  matchingProblems,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  comparePlaceholders,
  hasMismatch,
} from "@/features/question-bank/placeholders";

/**
 * publishProblem supplies immediate outline feedback for the server's publish
 * rules. A True / False / Not given with no answer, a "choose exactly N" whose
 * ticks are not N and a matching question with an item keyed to no option of
 * its own read "No correct answer set", the deck's outline warning (T-R6.9b).
 */
export function publishProblem(question: AdminQuestion, t: TFunction): string | null {
  if (question.points <= 0) return t("questionEditor.pointsError");
  if (
    ["single_choice", "multiple_choice", "true_false"].includes(question.type) &&
    !question.options?.some((option) => option.isCorrect)
  )
    return t("builder.correctOptionRequired");
  if (unkeyed(question)) return t("builder.noCorrectAnswer");
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

function unkeyed(question: AdminQuestion): boolean {
  const options = question.options ?? [];
  const ticked = options.filter((option) => option.isCorrect).length;
  switch (question.type) {
    case "tfng":
      return ticked !== 1;
    case "multiple_choice":
      return question.selectCount != null && ticked !== question.selectCount;
    case "matching": {
      const ids = new Set(options.map((option) => option.id));
      const items = question.matchItems ?? [];
      const keys = items.map((item) => item.correctOptionId);
      return (
        items.length === 0 ||
        keys.some((key) => !ids.has(key)) ||
        (question.matchAllowReuse !== true && new Set(keys).size < keys.length)
      );
    }
    default:
      return false;
  }
}

/**
 * draftUnkeyed reads the same rule on the editor's live draft, which the
 * schema keeps from saving: the selected question's outline row warns while
 * its draft is unkeyed, not only once a stored question is (QC-R6B-2).
 */
export function draftUnkeyed(values: QuestionValues): boolean {
  const ticked = values.options.filter((option) => option.isCorrect).length;
  switch (values.type) {
    case "tfng":
      return ticked !== 1;
    case "multiple_choice":
      return values.selectCount != null && ticked !== values.selectCount;
    case "matching": {
      const problems = matchingProblems(values);
      return (
        (values.matchItems ?? []).length === 0 ||
        problems.unset.size > 0 ||
        problems.taken.size > 0
      );
    }
    default:
      return false;
  }
}
