import { RichBlankPrompt } from "@/components/shared/content/RichBlankPrompt";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { trueFalseLabelKey } from "@/components/shared/content/trueFalse";
import { createContext, useContext, useId, useMemo, type ComponentProps } from "react";
import type { ExtraProps } from "react-markdown";
import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { nfc } from "@/lib/nfc";
import { useLargerTestText } from "@/lib/testText";
import { cn } from "@/lib/utils";
import { blankInputs } from "./blankInputs";
import { PaperSourceContext } from "../paper";
import { questionKind } from "../questionType";
import { sentenceGap, type SentenceGap } from "../sentenceGap";
import { useEngineWidthAtLeast } from "../width";
import { worth } from "../worth";
import type { Answer, StudentQuestion } from "../api";

const PROMPT = "leading-[1.6]! font-medium text-pretty";
const SENTENCE = "leading-[2.1]! font-medium text-pretty";
const GAP_SENTENCE = "leading-[1.8] font-medium text-pretty";
const GAP =
  "mx-[3px] inline-block max-w-full min-w-22 rounded-[7px] border-b-2 px-2.5 text-center leading-[1.6] break-words";
const SHORT_OPTION = 16;
const COLUMNS = ["grid-cols-1", "grid-cols-2", "grid-cols-3", "grid-cols-4"];
const HINT = "text-muted-fg -mt-2 text-sm";
const CAPTION = "text-muted-fg text-meta leading-normal";
const FIELD =
  "border-border bg-card text-fg placeholder:text-muted-fg border-[1.5px] outline-none focus:border-primary disabled:opacity-60";
const BLANK =
  "bg-muted text-fg placeholder:text-muted-fg mx-1 inline-block h-11 max-w-full rounded-t-[6px] rounded-b-none border-0 border-b-2 px-2 text-left align-baseline font-medium disabled:opacity-60 lg:h-9.5";

const PAPER_BLANKS = [
  "[&_.content-table-scroll_input]:bg-paper [&_.content-table-scroll_input]:text-paper-fg",
  "[&_.content-table-scroll_input]:border-[color-mix(in_oklab,var(--paper-fg)_30%,var(--paper))]",
  "[&_.content-table-scroll_input:focus]:border-paper-fg",
].join(" ");

function textSizes(larger: boolean) {
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
        field: "text-[length:var(--text-input)] lg:text-md",
        blank: "text-[length:var(--text-input)] lg:text-[length:inherit]",
      };
}

/**
 * QuestionBody is what a student answers, one question at a time: the prompt
 * at 17px, or a choice prompt's one gap as the deck's 18px sentence that
 * shows the chosen option, then the options as rows with letter markers, in
 * columns when they are all short and the question has no shared content;
 * the blanks as the deck's underlined fields inside an 18px sentence; or the
 * answer field with its word count. "Larger text in tests" raises the prompt
 * to 19px, both sentences to 20px and the answers to 17px. A type this page has no renderer
 * for draws nothing here and never calls `onAnswer`: QuestionSheet puts
 * UnknownType in its place.
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

function useBesideShared(questionId: string): boolean {
  const source = useContext(PaperSourceContext);
  return (
    source?.groups.some(
      (group) => group.stimuli.length > 0 && group.questionIds.includes(questionId),
    ) ?? false
  );
}

function columnsOf(
  options: NonNullable<StudentQuestion["options"]>,
  besideShared: boolean,
  wide: boolean,
): string {
  const short = options.every(
    (option) => option.content == null && [...option.text].length <= SHORT_OPTION,
  );
  if (!short || besideShared || options.length === 0) return "grid-cols-1";
  if (!wide) return "grid-cols-2";
  return COLUMNS[Math.min(4, options.length) - 1] ?? "grid-cols-1";
}

function Choice({ question, answer, onAnswer, disabled, larger }: Readonly<Props>) {
  const { t } = useTranslation();
  const options = question.options ?? [];
  const multiple = question.type === "multiple_choice";
  const instructionId = useId();
  const size = textSizes(larger);
  const wide = useEngineWidthAtLeast(768);
  const columns = columnsOf(options, useBesideShared(question.id), wide);
  const chosen = new Set(
    answer !== undefined && "optionIds" in answer ? answer.optionIds : [],
  );
  const gap = sentenceGap(question);

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
      {gap === null ? (
        <QuestionProse
          className={cn(size.prompt, PROMPT)}
          text={question.prompt}
          content={question.promptContent}
        />
      ) : (
        <GapSentence
          gap={gap}
          className={size.sentence}
          chosen={options.find((option) => chosen.has(option.id))}
          type={question.type}
        />
      )}
      <p id={instructionId} className={multiple ? HINT : "sr-only"}>
        {t(multiple ? "takeTest.chooseMultiple" : "takeTest.chooseSingle")}
      </p>
      <div
        className={cn("grid gap-2", columns)}
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
                "bg-card text-fg flex min-h-13 min-w-0 items-center gap-3 rounded-[11px] border-[1.5px] px-3.5 py-2.5 leading-[1.4]",
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
                  multiple ? "rounded-[7px]" : "rounded-full",
                  selected
                    ? "border-primary bg-primary text-primary-fg"
                    : "border-ring text-muted-fg",
                )}
              >
                {optionKey(index)}
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

function GapSentence({
  gap,
  className,
  chosen,
  type,
}: Readonly<{
  gap: SentenceGap;
  className: string;
  chosen: NonNullable<StudentQuestion["options"]>[number] | undefined;
  type: StudentQuestion["type"];
}>) {
  const { t } = useTranslation();
  if (chosen === undefined) {
    return (
      <p className={cn(className, GAP_SENTENCE)}>
        {gap.before}
        <span className={cn(GAP, "bg-muted border-ring")}>
          <span aria-hidden="true">{"\u00a0"}</span>
          <span className="sr-only">{t("takeTest.sentenceGap")}</span>
        </span>
        {gap.after}
      </p>
    );
  }
  const canonical = trueFalseLabelKey(type, chosen.text, chosen.content);
  return (
    <p className={cn(className, GAP_SENTENCE)}>
      {gap.before}
      <span className={cn(GAP, "bg-brand-soft border-brand")}>
        {canonical === null ? nfc(chosen.text) : t(canonical)}
      </span>
      {gap.after}
    </p>
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

  const tabled = useMemo(
    () => tableGaps(question.promptContent, false, new Set()),
    [question.promptContent],
  );

  const write = (blankId: string, value: string) =>
    onAnswer({ type: "fill_blank", values: { ...values, [blankId]: value } });
  const state: BlankState = {
    blanks,
    values,
    disabled,
    field: size.blank,
    single: (question.blanks ?? []).length === 1,
    write,
  };

  return (
    <BlankContext value={state}>
      <div className={cn("flex flex-col gap-4.5", PAPER_BLANKS)}>
        {question.promptContent != null ? (
          <RichBlankPrompt
            text={question.prompt}
            content={question.promptContent}
            blanks={question.blanks ?? []}
            className={cn(size.sentence, SENTENCE)}
            renderBlank={(blank) => (
              <BlankInput
                blank={blank}
                state={state}
                inTable={blank.gapId != null && tabled.has(blank.gapId)}
              />
            )}
          />
        ) : (
          <Markdown
            className={cn(size.sentence, SENTENCE)}
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
  single: boolean;
  write: (id: string, value: string) => void;
};

function tableGaps(node: unknown, inTable: boolean, found: Set<string>): Set<string> {
  if (Array.isArray(node)) {
    for (const item of node) tableGaps(item, inTable, found);
    return found;
  }
  if (typeof node !== "object" || node === null) return found;
  const record = node as Record<string, unknown>;
  const within = inTable || record["type"] === "table";
  if (within && record["type"] === "gap" && typeof record["id"] === "string")
    found.add(record["id"]);
  for (const value of Object.values(record)) tableGaps(value, within, found);
  return found;
}

const BlankContext = createContext<BlankState | null>(null);

function BlankSlot({ node, ...props }: ComponentProps<"span"> & ExtraProps) {
  const state = useContext(BlankContext);
  const ordinal = node?.properties["data-blank"];
  if (ordinal === undefined || ordinal === null || state === null)
    return <span {...props} />;
  const blank = state.blanks.get(String(ordinal));
  const token = `{{${String(ordinal)}}}`;
  if (blank === undefined) return <span>{token}</span>;
  const inTable = node?.properties["data-in-table"] != null;
  return <BlankInput blank={blank} state={state} inTable={inTable} />;
}

function BlankInput({
  blank,
  state,
  inTable,
}: Readonly<{
  blank: NonNullable<StudentQuestion["blanks"]>[number];
  state: BlankState;
  inTable: boolean;
}>) {
  const { t } = useTranslation();
  const value = state.values[blank.id] ?? "";
  return (
    <input
      className={cn(
        state.field,
        BLANK,
        state.single && !inTable ? "w-[min(100%,340px)]" : "w-32",
        value.trim() === "" ? "border-ring" : "border-primary",
      )}
      aria-label={t("takeTest.blankLabel", { n: blank.ordinal })}
      id={`answer-blank-${blank.id}`}
      value={value}
      disabled={state.disabled}
      placeholder={t("takeTest.blankPlaceholder")}
      autoComplete="off"
      spellCheck={false}
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
          autoComplete="off"
          spellCheck={false}
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
