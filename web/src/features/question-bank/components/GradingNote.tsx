import { useTranslation } from "react-i18next";
import { SquarePen, Zap } from "lucide-react";
import type { QuestionType } from "../questionSchema";

const EXTRA: Partial<Record<QuestionType, string>> = {
  fill_blank: "questionEditor.gradingNote.anyAccepted",
  multiple_choice: "questionEditor.gradingNote.exactlyTicked",
};

/**
 * GradingNote says under a question's answer how it is marked, as the deck
 * draws it: automatically for the choice types and fill in the blank, with a
 * second sentence for the two that have a rule, and by hand for short answer.
 */
export function GradingNote({ type }: Readonly<{ type: QuestionType }>) {
  const { t } = useTranslation();
  const manual = type === "short_answer";
  const Icon = manual ? SquarePen : Zap;
  const extra = EXTRA[type];
  return (
    <p className="text-muted-fg flex items-center gap-1.5 text-xs leading-normal">
      <Icon aria-hidden="true" className="size-3.25 shrink-0" />
      <span>
        {t(
          manual
            ? "questionEditor.gradingNote.manual"
            : "questionEditor.gradingNote.auto",
        )}
        {extra ? ` ${t(extra)}` : null}
      </span>
    </p>
  );
}
