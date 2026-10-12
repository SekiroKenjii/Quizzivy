import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Segmented } from "@/components/ui/segmented";
import type {
  QuestionType,
  QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  QUESTION_TYPES,
  retype,
  typeLocked,
} from "@/features/question-bank/questionType";

const KNOWN = new Set<string>(QUESTION_TYPES);

/**
 * QuestionTypeControl is the Question editor's segmented choice of §7's five
 * types. From 768px it keeps one row and scrolls sideways when the card is
 * narrower, as the deck's does; below 768px its options wrap, onto two rows
 * at 360px, and stretch to the row. While gaps bind answers
 * only "Fill in the blank" stays available, and the line under the control
 * says why. Choosing a type keeps the prompt and the points (`retype`).
 */
export function QuestionTypeControl({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const hint = useId();
  const locked = typeLocked(value);
  function choose(next: string) {
    if (next === value.type || !KNOWN.has(next)) return;
    onChange(retype(value, next as QuestionType));
  }
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Segmented
        label={t("questionEditor.typeLabel")}
        value={value.type}
        onChange={choose}
        scroll
        className="self-start max-[767px]:flex max-[767px]:w-full max-[767px]:flex-wrap max-[767px]:[&>button]:flex-auto max-[767px]:[&>button]:justify-center max-[767px]:[&>button]:px-2.5"
        options={QUESTION_TYPES.map((type) => {
          const blocked = locked && type !== "fill_blank" && type !== value.type;
          return {
            value: type,
            label: t(`questionEditor.type.${type}`),
            disabled: blocked,
            describedBy: blocked ? hint : undefined,
          };
        })}
      />
      {locked ? (
        <p id={hint} className="text-muted-fg text-xs">
          {t("questionEditor.richBlankSwitch")}
        </p>
      ) : null}
    </div>
  );
}
