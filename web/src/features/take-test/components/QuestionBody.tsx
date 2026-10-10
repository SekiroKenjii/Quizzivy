import { RichBlankPrompt } from "@/components/shared/content/RichBlankPrompt";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { createContext, useContext, useId, useMemo, type ComponentProps } from "react";
import type { ExtraProps } from "react-markdown";
import { useTranslation } from "react-i18next";
import { Check } from "lucide-react";
import { Markdown } from "@/components/shared/Markdown";
import { useLargerTestText } from "@/lib/testText";
import { cn } from "@/lib/utils";
import { blankInputs } from "./blankInputs";
import { questionKind } from "../questionType";
import { worth } from "../worth";
import type { Answer, StudentQuestion } from "../api";

const PROMPT = "leading-[1.6]! font-medium text-pretty";
const HINT = "text-muted-fg -mt-2 text-sm";
const CAPTION = "text-muted-fg text-meta leading-normal";
const FIELD =
  "border-border bg-card text-fg placeholder:text-muted-fg border-[1.5px] outline-none focus:border-primary disabled:opacity-60";

const PAPER_BLANKS = [
  "[&_.content-table-scroll_input]:bg-paper [&_.content-table-scroll_input]:text-paper-fg",
  "[&_.content-table-scroll_input]:border-[color-mix(in_oklab,var(--paper-fg)_30%,var(--paper))]",
  "[&_.content-table-scroll_input:focus]:border-paper-fg",
].join(" ");

function textSizes(larger: boolean) {
  return larger
    ? { prompt: "text-[1.1875rem]", answer: "text-lg", field: "text-lg" }
    : {
        prompt: "text-lg",
        answer: "text-md",
        field: "text-[length:var(--text-input)] lg:text-md",
      };
}

/**
 * QuestionBody is what a student answers, one question at a time: the prompt
 * at 17px, then the options as rows with letter markers, the blanks inside
 * the sentence, or the answer field with its word count. "Larger text in
 * tests" raises the prompt to 19px and the answers to 17px. A type this page
 * has no renderer for draws nothing here and never calls `onAnswer`:
 * QuestionSheet puts UnknownType in its place.
 */
export function QuestionBody({
  question,
  answer,
  onAnswer,
  disabled = false,
}: Readonly<{
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled?: boolean;
}>) {
  const larger = useLargerTestText();
  const props = { question, answer, onAnswer, disabled, larger };
  switch (questionKind(question)) {
    case "choice":
      return <Choice {...props} />;
    case "fill_blank":
      return <FillBlank {...props} />;
    case "short_answer":
      return <ShortAnswer {...props} />;
    case "unknown":
      return null;
  }
}

type Props = {
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled: boolean;
  larger: boolean;
};

function optionKey(index: number): string {
  return String.fromCharCode(65 + index);
}

function Choice({ question, answer, onAnswer, disabled, larger }: Readonly<Props>) {
  const { t } = useTranslation();
  const options = question.options ?? [];
  const multiple = question.type === "multiple_choice";
  const instructionId = useId();
  const size = textSizes(larger);
  const chosen = new Set(
    answer !== undefined && "optionIds" in answer ? answer.optionIds : [],
  );

  const toggle = (optionId: string) => {
    if (multiple) {
      const next = new Set(chosen);
      if (next.has(optionId)) next.delete(optionId);
      else next.add(optionId);
      onAnswer({ type: "choice", optionIds: [...next] });
      return;
    }
    onAnswer({ type: "choice", optionIds: [optionId] });
  };

  return (
    <div className="flex flex-col gap-4.5">
      <QuestionProse
        className={cn(size.prompt, PROMPT)}
        text={question.prompt}
        content={question.promptContent}
      />
      <p id={instructionId} className={multiple ? HINT : "sr-only"}>
        {t(multiple ? "takeTest.chooseMultiple" : "takeTest.chooseSingle")}
      </p>
      <div
        className="flex flex-col gap-2"
        role={multiple ? "group" : "radiogroup"}
        aria-label={t("takeTest.answerOptions")}
        aria-describedby={instructionId}
      >
        {options.map((option, index) => {
          const selected = chosen.has(option.id);
          return (
            <label
              key={option.id}
              className={cn(
                size.answer,
                "bg-card text-fg flex min-h-13 items-center gap-3 rounded-[11px] border-[1.5px] px-3.5 py-2.5 leading-[1.45]",
                "has-[:focus-visible]:outline-focus has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2",
                selected ? "border-primary" : "border-border",
                disabled ? "cursor-default" : "cursor-pointer",
                !disabled && !selected && "hover:border-ring",
              )}
            >
              <input
                type={multiple ? "checkbox" : "radio"}
                name={question.id}
                className="sr-only"
                checked={selected}
                disabled={disabled}
                onChange={() => toggle(option.id)}
              />
              <span
                aria-hidden="true"
                className={cn(
                  "grid size-6.5 flex-none place-items-center border-[1.5px] text-xs font-semibold",
                  multiple ? "rounded-[6px]" : "rounded-full",
                  selected
                    ? "border-primary bg-primary text-primary-fg"
                    : "border-ring text-muted-fg",
                )}
              >
                {selected ? <Check className="size-3.5" /> : optionKey(index)}
              </span>
              <span className="min-w-0 flex-1">
                <OptionText
                  text={option.text}
                  content={option.content}
                  type={question.type}
                />
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

function FillBlank({ question, answer, onAnswer, disabled, larger }: Readonly<Props>) {
  const { t } = useTranslation();
  const values = answer !== undefined && "values" in answer ? answer.values : {};
  const size = textSizes(larger);
  const blanks = useMemo(
    () => new Map(question.blanks?.map((blank) => [String(blank.ordinal), blank])),
    [question.blanks],
  );

  const write = (blankId: string, value: string) =>
    onAnswer({ type: "fill_blank", values: { ...values, [blankId]: value } });
  const state = { blanks, values, disabled, field: size.field, write };

  return (
    <BlankContext value={state}>
      <div className={cn("flex flex-col gap-4.5", PAPER_BLANKS)}>
        {question.promptContent != null ? (
          <RichBlankPrompt
            text={question.prompt}
            content={question.promptContent}
            blanks={question.blanks ?? []}
            className={cn(size.prompt, PROMPT)}
            renderBlank={(blank) => <BlankInput blank={blank} state={state} />}
          />
        ) : (
          <Markdown
            className={cn(size.prompt, PROMPT)}
            plugins={[blankInputs]}
            components={blankComponents}
          >
            {question.prompt}
          </Markdown>
        )}
        <p role="note" className={HINT}>
          {t(
            (question.blanks ?? []).some((blank) => blank.caseSensitive)
              ? "takeTest.blankRuleSensitive"
              : "takeTest.blankRuleInsensitive",
          )}
        </p>
      </div>
    </BlankContext>
  );
}

type BlankState = {
  blanks: Map<string, NonNullable<StudentQuestion["blanks"]>[number]>;
  values: Record<string, string>;
  disabled: boolean;
  field: string;
  write: (id: string, value: string) => void;
};

const BlankContext = createContext<BlankState | null>(null);

function BlankSlot({ node, ...props }: ComponentProps<"span"> & ExtraProps) {
  const state = useContext(BlankContext);
  const ordinal = node?.properties["data-blank"];
  if (ordinal === undefined || ordinal === null || state === null)
    return <span {...props} />;
  const blank = state.blanks.get(String(ordinal));
  const token = `{{${String(ordinal)}}}`;
  if (blank === undefined) return <span>{token}</span>;
  return <BlankInput blank={blank} state={state} />;
}

function BlankInput({
  blank,
  state,
}: Readonly<{
  blank: NonNullable<StudentQuestion["blanks"]>[number];
  state: BlankState;
}>) {
  const { t } = useTranslation();
  return (
    <input
      className={cn(
        state.field,
        FIELD,
        "rounded-ctl mx-1 my-1 inline-block h-11 w-32 max-w-full px-2.5 text-center align-middle font-normal lg:h-10",
      )}
      aria-label={t("takeTest.blankLabel", { n: blank.ordinal })}
      id={`answer-blank-${blank.id}`}
      value={state.values[blank.id] ?? ""}
      disabled={state.disabled}
      autoComplete="off"
      onChange={(event) => state.write(blank.id, event.target.value)}
    />
  );
}

const blankComponents = { span: BlankSlot };

function ShortAnswer({
  question,
  answer,
  onAnswer,
  disabled,
  larger,
}: Readonly<Props>) {
  const { t } = useTranslation();
  const value = answer !== undefined && "value" in answer ? String(answer.value) : "";
  const words = value.trim() === "" ? 0 : value.trim().split(/\s+/).length;
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
          rows={1}
          className={cn(
            size.field,
            FIELD,
            "field-sizing-content min-h-13 w-full resize-none rounded-[11px] px-3.5 leading-[1.45] disabled:cursor-not-allowed",
            "not-supports-[field-sizing:content]:min-h-24 not-supports-[field-sizing:content]:resize-y",
            larger ? "py-3" : "py-[13px]",
          )}
          value={value}
          disabled={disabled}
          placeholder={t("takeTest.answerPlaceholder")}
          aria-label={t("takeTest.yourAnswer")}
          onChange={(event) => onAnswer({ type: "text", value: event.target.value })}
        />
        <div className="flex items-baseline justify-between gap-3">
          <p className={cn(CAPTION, "tabular-nums")}>
            {t("takeTest.wordCount", { count: words })}
          </p>
          <p className={CAPTION}>{worth(question, t)}</p>
        </div>
      </div>
    </div>
  );
}
