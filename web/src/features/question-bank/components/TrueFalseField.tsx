import { useId } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { readTrueFalse, trueFalseOptions } from "../questionType";
import type { QuestionValues } from "../questionSchema";

type Option = QuestionValues["options"][number];

/**
 * TrueFalseField is a true/false question's answer: a radio group of two
 * cards, "True" and "False", in the reader's language. It reads the answer by
 * canonical text (readTrueFalse), not by position, and writes the two canonical
 * options "True" and "False" (DG-139), each keeping the id of the option it
 * replaces, so editing a legacy question whose teacher renamed them normalises
 * their texts; the ticked one is the correct answer.
 */
export function TrueFalseField({
  options,
  onChange,
}: Readonly<{ options: Option[]; onChange: (options: Option[]) => void }>) {
  const { t } = useTranslation();
  const heading = useId();
  const correctNote = useId();
  const name = useId();
  const answer = readTrueFalse(options);
  const choices = [
    { value: true, label: t("trueFalse.true") },
    { value: false, label: t("trueFalse.false") },
  ];
  return (
    <div className="flex flex-col gap-2.5">
      <p
        id={heading}
        className="text-[12.5px] font-medium in-data-[field-size=page]:text-[13px] in-data-[field-size=page]:leading-[19.5px]"
      >
        {t("questionEditor.trueFalseLabel")}{" "}
        <span className="text-muted-fg font-normal">
          · {t("questionEditor.trueFalseHint")}
        </span>
      </p>
      <span id={correctNote} className="sr-only">
        {t("questionEditor.correct")}
      </span>
      <div
        role="radiogroup"
        aria-labelledby={heading}
        className="grid grid-cols-[repeat(auto-fit,minmax(140px,1fr))] gap-2"
      >
        {choices.map((choice) => {
          const on = choice.value === answer.trueIsCorrect;
          return (
            <label
              key={String(choice.value)}
              className={cn(
                "text-fg flex min-h-11.5 min-w-0 cursor-pointer items-center gap-2.5 rounded-[10px] border-[1.5px] px-3.5 text-[14px] font-medium",
                "has-[:focus-visible]:outline-focus has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2",
                on ? "border-success bg-success-soft" : "border-border bg-background",
              )}
            >
              <input
                type="radio"
                name={name}
                className="sr-only"
                checked={on}
                aria-describedby={on ? correctNote : undefined}
                onChange={() => onChange(trueFalseOptions(choice.value, answer))}
              />
              <span
                aria-hidden="true"
                className={cn(
                  "grid size-4.5 flex-none place-items-center rounded-full border-2",
                  on ? "border-success" : "border-ring",
                )}
              >
                <span className={cn("size-2 rounded-full", on && "bg-success")} />
              </span>
              <span className="min-w-max flex-[1_1_auto]">{choice.label}</span>
              {on ? (
                <CircleCheck
                  aria-hidden="true"
                  className="text-success-ink size-4 flex-none"
                />
              ) : null}
            </label>
          );
        })}
      </div>
    </div>
  );
}
