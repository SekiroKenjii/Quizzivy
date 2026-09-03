import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { blankInputs } from "./blankInputs";
import type { Answer, StudentQuestion } from "../api";

/** What a student answers, one question at a time (S-05). */
export function QuestionBody({
  question,
  answer,
  onAnswer,
  disabled = false,
}: {
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  /** Read-only once the paper is locked; the answers stay legible. */
  disabled?: boolean;
}) {
  switch (question.type) {
    case "fill_blank":
      return (
        <FillBlank
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
        />
      );
    case "short_answer":
      return (
        <ShortAnswer
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
        />
      );
    default:
      return (
        <Choice
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
        />
      );
  }
}

type Props = {
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled: boolean;
};

/** A, B, C … the label the student and the teacher both refer to out loud. */
function optionKey(index: number): string {
  return String.fromCharCode(65 + index);
}

function Prompt({ children }: { children: string }) {
  return <Markdown className="text-base">{children}</Markdown>;
}

/**
 * single_choice, multiple_choice and true_false, which differ only in how many
 * may be chosen.
 */
function Choice({ question, answer, onAnswer, disabled }: Props) {
  const { t } = useTranslation();
  const options = question.options ?? [];
  const multiple = question.type === "multiple_choice";
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
      <Prompt>{question.prompt}</Prompt>

      <div
        className="space-y-2.5"
        role={multiple ? "group" : "radiogroup"}
        aria-label={t("takeTest.answerOptions")}
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
                  "grid size-6 shrink-0 place-content-center rounded-sm border text-xs font-semibold",
                  selected
                    ? "bg-primary text-primary-foreground border-transparent"
                    : "text-muted-foreground",
                )}
              >
                {optionKey(index)}
              </span>
              <span className="text-base">{option.text}</span>
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
function FillBlank({ question, answer, onAnswer, disabled }: Props) {
  const { t } = useTranslation();
  const values = answer !== undefined && "values" in answer ? answer.values : {};
  const blanks = question.blanks ?? [];

  const write = (blankId: string, value: string) =>
    onAnswer({ type: "fill_blank", values: { ...values, [blankId]: value } });

  return (
    <div className="space-y-4">
      <Markdown
        className="text-base"
        plugins={[blankInputs]}
        components={{
          span: (props) => {
            const ordinal = props.node?.properties?.["data-blank"];
            if (ordinal === undefined || ordinal === null) return <span {...props} />;
            const blank = blanks.find((b) => String(b.ordinal) === String(ordinal));
            if (blank === undefined) {
              // A placeholder with no blank behind it.
              const orphan = `{{${String(ordinal)}}}`;
              return <span>{orphan}</span>;
            }
            return (
              <input
                className="border-input focus-visible:ring-ring mx-1 inline-block h-9 w-28 rounded-md border px-3 text-center align-middle text-sm focus-visible:ring-2 focus-visible:outline-none"
                aria-label={t("takeTest.blankLabel", { n: blank.ordinal })}
                value={values[blank.id] ?? ""}
                disabled={disabled}
                onChange={(event) => write(blank.id, event.target.value)}
              />
            );
          },
        }}
      >
        {question.prompt}
      </Markdown>
    </div>
  );
}

function ShortAnswer({ question, answer, onAnswer, disabled }: Props) {
  const { t } = useTranslation();
  const value = answer !== undefined && "value" in answer ? String(answer.value) : "";
  // Whitespace-separated, which is what "18 từ" means to a student writing English.
  const words = value.trim() === "" ? 0 : value.trim().split(/\s+/).length;

  return (
    <div className="space-y-4">
      <Prompt>{question.prompt}</Prompt>
      <Textarea
        className="min-h-36 leading-relaxed"
        value={value}
        disabled={disabled}
        aria-label={t("takeTest.yourAnswer")}
        onChange={(event) => onAnswer({ type: "text", value: event.target.value })}
      />
      <p className="text-muted-foreground text-right text-xs tabular-nums">
        {t("takeTest.wordCount", { count: words })}
      </p>
    </div>
  );
}
