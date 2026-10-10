import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Plus, TriangleAlert, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import { cn } from "@/lib/utils";
import { OptionField } from "./OptionField";
import { optionLetter } from "../questionType";
import {
  choiceProblems,
  MAX_OPTIONS,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";

type Option = QuestionValues["options"][number];

interface OptionsEditorProps {
  options: Option[];
  multiple: boolean;
  onChange: (options: Option[]) => void;
}

/**
 * OptionsEditor is a single- or multiple-choice question's options, lettered
 * A–H, with the correct answer marked on the row where it is written. "Add
 * option" stops at eight with an error toast and remove at two. For multiple
 * choice it says when nothing is ticked, which blocks saving, and when only
 * one is, which is a warning only: the server takes one correct answer.
 */
export function OptionsEditor({
  options,
  multiple,
  onChange,
}: Readonly<OptionsEditorProps>) {
  const { t } = useTranslation();
  const [editorEpoch, setEditorEpoch] = useState(0);
  const heading = useId();
  const note = useId();
  const name = useId();
  const problems = choiceProblems({
    type: multiple ? "multiple_choice" : "single_choice",
    options,
  });
  const sentence = multiple ? multipleSentence(problems) : null;

  function setCorrect(index: number, correct: boolean) {
    onChange(
      options.map((option, i) => ({
        ...option,
        isCorrect: nextCorrect(multiple, i === index, correct, option.isCorrect),
      })),
    );
  }

  function add() {
    if (options.length >= MAX_OPTIONS) {
      toast.error(t("questionEditor.maxOptionsToast", { count: MAX_OPTIONS }));
      return;
    }
    onChange([...options, { id: null, text: "", isCorrect: false }]);
  }

  return (
    <div className="flex flex-col gap-2">
      <p id={heading} className="text-[12.5px] font-medium">
        {t("questionEditor.options")}{" "}
        <span className="text-muted-fg font-normal">
          ·{" "}
          {t(
            multiple ? "questionEditor.optionsHintMulti" : "questionEditor.optionsHint",
          )}
        </span>
      </p>

      <div
        role="group"
        aria-labelledby={heading}
        aria-describedby={sentence ? note : undefined}
        className="flex flex-col gap-2"
      >
        {options.map((option, index) => {
          const letter = optionLetter(index);
          return (
            <div
              key={option.id ?? `${editorEpoch}-${index}`}
              className={cn(
                "flex items-center gap-2.5 rounded-[9px] border py-1 pr-1.5 pl-1",
                option.isCorrect
                  ? "border-success bg-success-soft"
                  : "border-input bg-background",
              )}
            >
              <label className="has-[:focus-visible]:outline-focus grid size-8 flex-none cursor-pointer place-items-center rounded-[7px] has-[:focus-visible]:outline-2">
                <input
                  type={multiple ? "checkbox" : "radio"}
                  name={name}
                  className="peer sr-only"
                  checked={option.isCorrect}
                  aria-label={t("questionEditor.markCorrect", { letter })}
                  onChange={(event) => setCorrect(index, event.target.checked)}
                />
                <MarkVisual multiple={multiple} on={option.isCorrect} />
              </label>
              <span
                aria-hidden="true"
                className="text-muted-fg w-4.5 flex-none text-[12.5px] font-semibold"
              >
                {letter}
              </span>
              <OptionField
                text={option.text}
                content={option.content ?? null}
                index={index}
                label={t("questionEditor.optionLetter", { letter })}
                bare
                onChange={(text, content) =>
                  onChange(
                    options.map((current, i) =>
                      i === index ? { ...current, text, content } : current,
                    ),
                  )
                }
              />
              {option.isCorrect ? (
                <span
                  aria-hidden="true"
                  className="text-success-ink flex-none text-xs leading-normal font-medium whitespace-nowrap"
                >
                  {t("questionEditor.correct")}
                </span>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="text-muted-fg size-7 flex-none rounded-[6px] in-data-[scale=deck]:rounded-[6px]"
                aria-label={t("questionEditor.removeOptionLetter", { letter })}
                disabled={options.length <= 2}
                onClick={() => {
                  setEditorEpoch((epoch) => epoch + 1);
                  onChange(options.filter((_, i) => i !== index));
                }}
              >
                <X aria-hidden="true" className="size-3.5" />
              </Button>
            </div>
          );
        })}
      </div>

      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-fg h-7.5 self-start rounded-[7px] px-2.5 text-[12.5px] font-normal in-data-[scale=deck]:rounded-[7px] in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:text-[12.5px]"
        onClick={add}
      >
        <Plus aria-hidden="true" className="size-3.5" />
        {t("questionEditor.addOption")}
      </Button>

      {sentence ? (
        <p
          id={note}
          className="bg-warning-soft flex items-start gap-2 rounded-lg px-2.5 py-2 text-[12.5px] leading-normal"
        >
          <TriangleAlert
            aria-hidden="true"
            className="text-warning-ink mt-0.5 size-3.5 flex-none"
          />
          {t(sentence)}
        </p>
      ) : null}
    </div>
  );
}

function multipleSentence(problems: ReturnType<typeof choiceProblems>) {
  if (problems.noneCorrect) return "questionEditor.tickTwo";
  if (problems.oneCorrectOfMany) return "questionEditor.onlyOneTicked";
  return null;
}

function MarkVisual({ multiple, on }: Readonly<{ multiple: boolean; on: boolean }>) {
  if (multiple)
    return (
      <span
        aria-hidden="true"
        className={cn(
          "text-success-foreground grid size-4.5 place-items-center rounded-[5px] border-2",
          on ? "border-success bg-success" : "border-ring bg-transparent",
        )}
      >
        {on ? <Check className="size-3" /> : null}
      </span>
    );
  return (
    <span
      aria-hidden="true"
      className={cn(
        "grid size-4.5 place-items-center rounded-full border-2",
        on ? "border-success" : "border-ring",
      )}
    >
      <span className={cn("size-2 rounded-full", on && "bg-success")} />
    </span>
  );
}

function nextCorrect(
  multiple: boolean,
  isTarget: boolean,
  correct: boolean,
  current: boolean,
): boolean {
  if (!multiple) return isTarget;
  return isTarget ? correct : current;
}
