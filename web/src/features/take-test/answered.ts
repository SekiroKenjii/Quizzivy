import type { Answer, StudentQuestion } from "./api";

/**
 * Whether an answer says anything. S-06's dots and the review's counts both
 * read this, so "đã trả lời" means one thing everywhere: a choice with a
 * selection, text with anything but whitespace, a true/false that was actually
 * picked, and a fill_blank with every blank filled.
 *
 * Every blank, not any. Grading gives per-blank credit (O-17), so one blank of
 * four is a quarter of the marks and three quarters still on the table -- and
 * the screen whose whole job is "check before you submit" is the wrong place
 * to call that done. The deck's vocabulary has three states and no partial
 * one, so the question goes to "chưa làm", which is what gets it counted in
 * the confirm dialog. Nagging the student who genuinely knows only one blank
 * is the cheaper mistake, because the other one is silent.
 *
 * It takes the question because the answer alone cannot say how many blanks
 * there are: `values` carries only the ones that have been typed into.
 */
export function answered(
  question: StudentQuestion,
  answer: Answer | undefined,
): boolean {
  if (answer === undefined) return false;
  switch (answer.type) {
    case "choice":
      return answer.optionIds.length > 0;
    case "true_false":
      return typeof answer.value === "boolean";
    case "fill_blank": {
      // No blanks at all is a paper publish refuses to make. If one ever
      // reaches a student, "not answered" is the harmless way to be wrong.
      const blanks = question.blanks ?? [];
      return (
        blanks.length > 0 &&
        blanks.every((blank) => (answer.values[blank.id] ?? "").trim() !== "")
      );
    }
    case "text":
      return answer.value.trim() !== "";
  }
}
