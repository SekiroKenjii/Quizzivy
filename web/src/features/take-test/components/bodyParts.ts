import type { Answer, StudentQuestion } from "../api";

/** The parts every answer renderer of QuestionBody shares. */

export const PROMPT = "leading-[1.6]! font-medium text-pretty";
export const CAPTION = "text-muted-fg text-meta leading-normal";
export const FIELD =
  "border-border bg-card text-fg placeholder:text-muted-fg border-[1.5px] outline-none focus:border-primary disabled:opacity-60";

export type BodyProps = {
  question: StudentQuestion;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled: boolean;
  larger: boolean;
};

export function textSizes(larger: boolean) {
  return larger
    ? {
        prompt: "text-[1.1875rem]",
        sentence: "text-xl",
        answer: "text-lg",
        field: "text-lg",
        blank: "text-[length:inherit]",
      }
    : {
        prompt: "text-lg",
        sentence: "text-[1.125rem]",
        answer: "text-md",
        field: "text-[length:var(--text-input)] engine-lg:text-md",
        blank: "text-[length:var(--text-input)] engine-lg:text-[length:inherit]",
      };
}
