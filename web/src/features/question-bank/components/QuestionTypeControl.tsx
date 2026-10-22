import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Segmented } from "@/components/ui/segmented";
import type {
  QuestionType,
  QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  editorType,
  QUESTION_TYPES,
  retype,
  typeLocked,
} from "@/features/question-bank/questionType";

const KNOWN = new Set<string>(QUESTION_TYPES);

/**
 * QuestionTypeControl is the Question editor's segmented choice of the seven
 * types it offers (T-R6.9a); a tfng question shows as True / False, whose
 * switch makes it one. Its options wrap when the card is narrower than the
 * seven, so every type stays in view (QC-R6AC-2); below 768px they also
 * stretch to the row. While gaps bind answers
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
  const shown = editorType(value.type);
  function choose(next: string) {
    if (next === shown || !KNOWN.has(next)) return;
    onChange(retype(value, next as QuestionType));
  }
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Segmented
        label={t("questionEditor.typeLabel")}
        value={shown}
        onChange={choose}
        className="flex max-w-full flex-wrap self-start max-[767px]:w-full max-[767px]:[&>button]:flex-auto max-[767px]:[&>button]:justify-center max-[767px]:[&>button]:px-2.5"
        options={QUESTION_TYPES.map((type) => {
          const blocked = locked && type !== "fill_blank" && type !== shown;
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
