import { useId } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import {
  readTfng,
  readTrueFalse,
  tfngOptions,
  trueFalseOptions,
  withNotGiven,
  type TfngAnswer,
} from "../questionType";
import type { QuestionValues } from "../questionSchema";

/**
 * TrueFalseField is a true/false question's answer: a radio group of cards,
 * "True" and "False", in the reader's language, and the switch "Include “Not
 * given”" (DG-113). On, the question is stored as `tfng` and a third card,
 * "Not given", joins them; off, it is `true_false`, and an answer of "Not
 * given" becomes False, as the deck draws it. A true/false answer is read by
 * canonical text (readTrueFalse), not by position, and written as the two
 * canonical options "True" and "False" (DG-139), each keeping the id of the
 * option it replaces, so editing a legacy question whose teacher renamed them
 * normalises their texts. A tfng answer is read by position, the server's
 * fixed order. The ticked card is the correct answer. The cards share a row,
 * each at least 140px, and a card that wraps takes the row's width rather than
 * one column of it (QC-R6AC-7).
 */
export function TrueFalseField({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const heading = useId();
  const correctNote = useId();
  const name = useId();
  const notGiven = value.type === "tfng";
  const options = value.options;
  const answer = readTrueFalse(options);
  const current = currentAnswer(notGiven, options);
  const choices: ReadonlyArray<{ value: TfngAnswer; label: string }> = [
    { value: "true", label: t("trueFalse.true") },
    { value: "false", label: t("trueFalse.false") },
    ...(notGiven
      ? [{ value: "not_given" as const, label: t("trueFalse.notGiven") }]
      : []),
  ];
  function choose(next: TfngAnswer) {
    onChange({
      ...value,
      options: notGiven
        ? tfngOptions(next, options)
        : trueFalseOptions(next === "true", answer),
    });
  }
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
        className="flex flex-wrap gap-2 [&>label]:flex-[1_1_140px]"
      >
        {choices.map((choice) => {
          const on = choice.value === current;
          return (
            <label
              key={choice.value}
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
                onChange={() => choose(choice.value)}
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
              <span className="min-w-0 flex-[1_1_auto] break-words">
                {choice.label}
              </span>
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
      <label className="flex cursor-pointer items-center gap-2.5 text-[13px]">
        <Switch
          size="sm"
          checked={notGiven}
          onCheckedChange={(on) => onChange(withNotGiven(value, on))}
        />
        <span>
          {t("questionEditor.includeNotGiven")}{" "}
          <span className="text-muted-fg">
            · {t("questionEditor.includeNotGivenHint")}
          </span>
        </span>
      </label>
    </div>
  );
}

function currentAnswer(
  notGiven: boolean,
  options: QuestionValues["options"],
): TfngAnswer {
  if (notGiven) return readTfng(options);
  return readTrueFalse(options).trueIsCorrect ? "true" : "false";
}
