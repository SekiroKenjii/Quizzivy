import { useTranslation } from "react-i18next";
import { SquarePen, Zap } from "lucide-react";
import type { QuestionType } from "../questionSchema";

const EXTRA: Partial<Record<QuestionType, string>> = {
  fill_blank: "questionEditor.gradingNote.anyAccepted",
  multiple_choice: "questionEditor.gradingNote.exactlyTicked",
  matching: "questionEditor.gradingNote.itemShare",
};

const MANUAL: Partial<Record<QuestionType, string>> = {
  short_answer: "questionEditor.gradingNote.manual",
  essay: "questionEditor.gradingNote.band",
};

/**
 * GradingNote says under a question's answer how it is marked, as the deck
 * draws it: automatically for the choice types, fill in the blank and
 * matching, with a second sentence for the three that have a rule; by hand
 * for short answer; and for an essay "Graded by you on a 3–9 band" (T-R6.9a).
 * "Choose exactly N" scores each correct pick, so its rule says that instead.
 */
export function GradingNote({
  type,
  selectCount = null,
}: Readonly<{ type: QuestionType; selectCount?: number | null | undefined }>) {
  const { t } = useTranslation();
  const manual = MANUAL[type];
  const Icon = manual ? SquarePen : Zap;
  const extra =
    type === "multiple_choice" && selectCount != null
      ? "questionEditor.gradingNote.perPick"
      : EXTRA[type];
  return (
    <p className="text-muted-fg flex items-center gap-1.5 text-xs leading-normal">
      <Icon aria-hidden="true" className="size-3.25 shrink-0" />
      <span>
        {t(manual ?? "questionEditor.gradingNote.auto")}
        {extra ? ` ${t(extra)}` : null}
      </span>
    </p>
  );
}
