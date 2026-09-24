import { RichBlankPrompt } from "@/components/shared/content/RichBlankPrompt";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import {
  createContext,
  useContext,
  useId,
  useMemo,
  type ComponentProps,
  type ReactNode,
} from "react";
import type { ExtraProps } from "react-markdown";
import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { blankInputs } from "./blankInputs";
import { worth } from "../worth";
import type { Answer, StudentQuestion } from "../api";

/** What a student answers, one question at a time (S-05). */
export function QuestionBody({
  question,
  answer,
  onAnswer,
  disabled = false,
  action,
}: Readonly<{
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  /** Read-only once the paper is locked; the answers stay legible. */
  disabled?: boolean;
  action?: ReactNode;
}>) {
  switch (question.type) {
    case "fill_blank":
      return (
        <FillBlank
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
          action={action}
        />
      );
    case "short_answer":
      return (
        <ShortAnswer
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
          action={action}
        />
      );
    default:
      return (
        <Choice
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
          action={action}
        />
      );
  }
}

type Props = {
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled: boolean;
  action?: ReactNode;
};

/** A, B, C … the label the student and the teacher both refer to out loud. */
function optionKey(index: number): string {
  return String.fromCharCode(65 + index);
}

function Prompt({
  children,
  content,
  action,
}: Readonly<{
  children: string;
  content: StudentQuestion["promptContent"];
  action?: ReactNode;
}>) {
  return (
    <div className="flex items-start gap-3">
      <QuestionProse
        className="min-w-0 flex-1 text-base"
        text={children}
        content={content}
      />
      {action}
    </div>
  );
}

/**
 * single_choice, multiple_choice and true_false, which differ only in how many
 * may be chosen.
 */
function Choice({ question, answer, onAnswer, disabled, action }: Readonly<Props>) {
  const { t } = useTranslation();
  const options = question.options ?? [];
  const multiple = question.type === "multiple_choice";
  const instructionId = useId();
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
    <div className="space-y-4">
      <Prompt action={action} content={question.promptContent}>
        {question.prompt}
      </Prompt>
      <p id={instructionId} className="text-muted-foreground text-sm">
        {t(multiple ? "takeTest.chooseMultiple" : "takeTest.chooseSingle")}
      </p>
      <div
        className="space-y-2.5"
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
                "group flex min-h-12 cursor-pointer items-start gap-3 rounded-lg border px-4 py-3.5",
                "transition-colors",
                disabled ? "cursor-default" : "hover:bg-accent",
                selected &&
                  "border-foreground bg-accent shadow-[inset_0_0_0_1px_var(--color-foreground)]",
                "has-[:focus-visible]:ring-ring has-[:focus-visible]:ring-2",
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
                  "grid size-6 shrink-0 place-content-center border text-xs font-semibold",
                  multiple ? "rounded-sm" : "rounded-full",
                  selected
                    ? "bg-primary text-primary-foreground border-transparent"
                    : "text-muted-foreground",
                )}
              >
                {optionKey(index)}
              </span>
              <span className="text-base">
                <OptionText text={option.text} content={option.content} />
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

/**
 * The inputs go where the placeholders are, inside the rendered Markdown, so
 * the sentence reads as a sentence rather than as a prompt followed by a list
 * of boxes.
 */
function FillBlank({ question, answer, onAnswer, disabled, action }: Readonly<Props>) {
  const values = answer !== undefined && "values" in answer ? answer.values : {};
  const blanks = useMemo(
    () => new Map(question.blanks?.map((blank) => [String(blank.ordinal), blank])),
    [question.blanks],
  );

  const write = (blankId: string, value: string) =>
    onAnswer({ type: "fill_blank", values: { ...values, [blankId]: value } });

  return (
    <BlankContext value={{ blanks, values, disabled, write }}>
      <div className="flex items-start gap-3">
        {question.promptContent != null ? (
          <RichBlankPrompt
            text={question.prompt}
            content={question.promptContent}
            blanks={question.blanks ?? []}
            className="min-w-0 flex-1 text-base"
            renderBlank={(blank) => (
              <BlankInput blank={blank} state={{ blanks, values, disabled, write }} />
            )}
          />
        ) : (
          <Markdown
            className="min-w-0 flex-1 text-base"
            plugins={[blankInputs]}
            components={blankComponents}
          >
            {question.prompt}
          </Markdown>
        )}
        {action}
      </div>
    </BlankContext>
  );
}

type BlankState = {
  blanks: Map<string, NonNullable<StudentQuestion["blanks"]>[number]>;
  values: Record<string, string>;
  disabled: boolean;
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
      className="border-input focus-visible:ring-ring mx-1 my-1 inline-block h-11 w-32 max-w-full rounded-md border px-3 text-center align-middle text-[length:var(--text-input)] focus-visible:ring-2 focus-visible:outline-none lg:h-9 lg:text-sm"
      aria-label={t("takeTest.blankLabel", { n: blank.ordinal })}
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
  action,
}: Readonly<Props>) {
  const { t } = useTranslation();
  const value = answer !== undefined && "value" in answer ? String(answer.value) : "";
  // Whitespace-separated, which is what "18 từ" means to a student writing English.
  const words = value.trim() === "" ? 0 : value.trim().split(/\s+/).length;

  return (
    <div className="space-y-4">
      <Prompt action={action} content={question.promptContent}>
        {question.prompt}
      </Prompt>
      <Textarea
        className="min-h-36 leading-relaxed"
        value={value}
        disabled={disabled}
        aria-label={t("takeTest.yourAnswer")}
        onChange={(event) => onAnswer({ type: "text", value: event.target.value })}
      />
      <div className="flex items-center justify-between gap-3 lg:justify-end">
        <p className="text-muted-foreground text-xs lg:hidden">{worth(question, t)}</p>
        <p className="text-muted-foreground text-xs tabular-nums">
          {t("takeTest.wordCount", { count: words })}
        </p>
      </div>
    </div>
  );
}
