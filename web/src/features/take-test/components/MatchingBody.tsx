import { useContext, useRef, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import { CAPTION, PROMPT, textSizes, type BodyProps } from "./bodyParts";
import { PaperSourceContext } from "../paper";
import { useEngineWidthAtLeast } from "../width";
import type { StudentQuestion } from "../api";

type Option = NonNullable<StudentQuestion["options"]>[number];
type Item = NonNullable<StudentQuestion["matchItems"]>[number];

/**
 * MatchingBody is a matching question as the deck's paper s9 draws it: the
 * prompt, "{k} of {n} matched", then one card per item with its key, its
 * text and a radiogroup of chips, one per option key. A matched card takes
 * the primary border. Tapping a chip pairs it, tapping the chosen chip clears
 * it. Without reuse, a chip another item holds is drawn faded, says which
 * item holds it, and choosing it moves it here. When the options carry text,
 * the question has no shared content beside it and the engine is at least
 * 1000 wide, a 220px card lists the options once, each with the items that
 * use it. Keys are shown as authored (DG-194). The arrows move between a
 * card's chips and Space or Enter chooses, so moving through a row never
 * takes a chip from another item.
 */
export function MatchingBody({
  question,
  answer,
  onAnswer,
  disabled,
  larger,
}: Readonly<BodyProps>) {
  const { t } = useTranslation();
  const size = textSizes(larger);
  const options = question.options ?? [];
  const items = question.matchItems ?? [];
  const reuse = question.matchAllowReuse === true;
  const pairs = answer?.type === "matching" ? answer.pairs : {};
  const known = new Set(options.map((option) => option.id));
  const matched = items.filter((item) => known.has(pairs[item.id] ?? "")).length;
  const beside = useBesideShared(question.id);
  const wide = useEngineWidthAtLeast(1000);
  const roomy = useEngineWidthAtLeast(1024);
  const withCard =
    wide && !beside && options.some((option) => option.text.trim() !== "");

  const holders = (optionId: string) =>
    items.filter((item) => pairs[item.id] === optionId).map((item) => item.key);

  const choose = (itemId: string, optionId: string) => {
    const next = { ...pairs };
    if (next[itemId] === optionId) {
      delete next[itemId];
    } else {
      if (!reuse) {
        for (const [other, held] of Object.entries(next)) {
          if (held === optionId) delete next[other];
        }
      }
      next[itemId] = optionId;
    }
    onAnswer({ type: "matching", pairs: next });
  };

  return (
    <div className="flex flex-col gap-4.5">
      <QuestionProse
        className={cn(size.prompt, PROMPT)}
        text={question.prompt}
        content={question.promptContent}
      />
      <div
        className={cn(
          "grid items-start gap-4",
          withCard ? "grid-cols-[220px_minmax(0,1fr)]" : "grid-cols-1",
        )}
      >
        {withCard && (
          <ul
            aria-label={t("takeTest.matching.options")}
            className="bg-card overflow-hidden rounded-xl border"
          >
            {options.map((option, index) => {
              const by = holders(option.id);
              return (
                <li
                  key={option.id}
                  className={cn(
                    "flex items-center gap-2.5 px-3.5 py-[9px] text-sm",
                    index > 0 && "border-t",
                  )}
                >
                  <span className="bg-muted grid h-5.5 min-w-5.5 flex-none place-items-center rounded-md px-1 text-xs font-bold">
                    {option.key}
                  </span>
                  <span className="min-w-0 flex-1 break-words">{nfc(option.text)}</span>
                  {by.length > 0 && (
                    <span className="text-success-ink text-xs font-semibold whitespace-nowrap">
                      <span className="sr-only">{t("takeTest.matching.usedBy")} </span>
                      {by.join(", ")}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        <div className="flex min-w-0 flex-col gap-2.5">
          <p className={cn(CAPTION, "tabular-nums")} aria-live="polite">
            {t("takeTest.matching.done", { k: matched, n: items.length })}
          </p>
          {items.map((item) => (
            <ItemCard
              key={item.id}
              item={item}
              options={options}
              chosen={pairs[item.id]}
              holdersOf={
                reuse ? () => [] : (id) => holders(id).filter((key) => key !== item.key)
              }
              disabled={disabled}
              roomy={roomy}
              textClass={size.answer}
              onChoose={(optionId) => choose(item.id, optionId)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function useBesideShared(questionId: string): boolean {
  const source = useContext(PaperSourceContext);
  return (
    source?.groups.some(
      (group) => group.stimuli.length > 0 && group.questionIds.includes(questionId),
    ) ?? false
  );
}

function ItemCard({
  item,
  options,
  chosen,
  holdersOf,
  disabled,
  roomy,
  textClass,
  onChoose,
}: Readonly<{
  item: Item;
  options: Option[];
  chosen: string | undefined;
  holdersOf: (optionId: string) => string[];
  disabled: boolean;
  roomy: boolean;
  textClass: string;
  onChoose: (optionId: string) => void;
}>) {
  const { t } = useTranslation();
  const chips = useRef<(HTMLButtonElement | null)[]>([]);
  const isMatched = options.some((option) => option.id === chosen);
  const focusable = Math.max(
    0,
    options.findIndex((option) => option.id === chosen),
  );

  const move = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const last = options.length - 1;
    const to: Record<string, number> = {
      ArrowRight: index === last ? 0 : index + 1,
      ArrowDown: index === last ? 0 : index + 1,
      ArrowLeft: index === 0 ? last : index - 1,
      ArrowUp: index === 0 ? last : index - 1,
      Home: 0,
      End: last,
    };
    const target = to[event.key];
    if (target === undefined) return;
    event.preventDefault();
    chips.current[target]?.focus();
  };

  return (
    <div
      className={cn(
        "bg-card flex flex-wrap items-center gap-x-3 gap-y-2.5 rounded-[11px] border-[1.5px] px-3 py-2.5",
        isMatched ? "border-primary" : "border-border",
      )}
    >
      <span className="flex min-w-0 flex-[1_1_170px] items-center gap-2.5">
        <span className="text-muted-fg text-[13px] font-bold">{item.key}</span>
        <span className={cn(textClass, "min-w-0 font-medium break-words")}>
          {nfc(item.text)}
        </span>
      </span>
      <span
        role="radiogroup"
        aria-label={t("takeTest.matching.item", {
          key: item.key,
          text: nfc(item.text),
        })}
        className="flex flex-wrap gap-1.5"
      >
        {options.map((option, index) => {
          const on = option.id === chosen;
          const elsewhere = on ? [] : holdersOf(option.id);
          const label =
            elsewhere.length > 0
              ? t("takeTest.matching.chipUsed", {
                  key: option.key,
                  by: elsewhere.join(", "),
                })
              : option.key;
          return (
            <button
              key={option.id}
              ref={(node) => {
                chips.current[index] = node;
              }}
              type="button"
              role="radio"
              aria-checked={on}
              aria-label={label}
              title={elsewhere.length > 0 ? label : undefined}
              tabIndex={index === focusable ? 0 : -1}
              disabled={disabled}
              onClick={() => onChoose(option.id)}
              onKeyDown={(event) => move(event, index)}
              className={cn(
                "grid place-items-center rounded-[9px] border-[1.5px] px-1.5 py-0 text-sm font-semibold",
                roomy ? "h-9.5 min-w-9.5" : "h-11 min-w-11",
                on && "border-primary bg-primary text-primary-fg",
                !on && elsewhere.length === 0 && "border-border bg-card text-fg",
                !on && elsewhere.length > 0 && "border-border/50 bg-card text-fg/50",
                disabled ? "cursor-default" : "cursor-pointer",
                !disabled && !on && "hover:border-ring",
              )}
            >
              {option.key}
            </button>
          );
        })}
      </span>
    </div>
  );
}
