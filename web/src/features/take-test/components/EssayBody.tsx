import { useId } from "react";
import { useTranslation } from "react-i18next";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { cn } from "@/lib/utils";
import { CAPTION, FIELD, PROMPT, textSizes, type BodyProps } from "./bodyParts";
import { WordCount } from "./WordCount";
import { worth } from "../worth";

/**
 * EssayBody is an essay's prompt and its field: a textarea that starts at
 * twelve lines and grows with the text, the live word count under it, and
 * what the essay is worth. It writes a text answer, so it autosaves and keeps
 * its local draft as a short answer does, and pasting is refused by the
 * integrity monitor whenever the policy refuses it on the rest of the paper.
 */
export function EssayBody({
  question,
  answer,
  onAnswer,
  disabled,
  larger,
}: Readonly<BodyProps>) {
  const { t } = useTranslation();
  const value = answer?.type === "text" ? answer.value : "";
  const countId = useId();
  const size = textSizes(larger);

  return (
    <div className="flex flex-col gap-4.5">
      <QuestionProse
        className={cn(size.prompt, PROMPT)}
        text={question.prompt}
        content={question.promptContent}
      />
      <div className="flex flex-col gap-1.5">
        <textarea
          rows={12}
          className={cn(
            size.field,
            FIELD,
            "field-sizing-content min-h-[calc(12lh+1.75rem)] w-full resize-none rounded-[11px] px-3.5 py-3 leading-[1.6] disabled:cursor-not-allowed",
            "not-supports-[field-sizing:content]:resize-y",
          )}
          value={value}
          disabled={disabled}
          placeholder={t("takeTest.essayPlaceholder")}
          aria-label={t("takeTest.yourEssay")}
          aria-describedby={countId}
          autoComplete="off"
          spellCheck={false}
          onChange={(event) => onAnswer({ type: "text", value: event.target.value })}
        />
        <div className="flex items-baseline justify-between gap-3">
          <WordCount id={countId} value={value} limit={undefined} />
          <p className={CAPTION}>{worth(question, t)}</p>
        </div>
      </div>
    </div>
  );
}
