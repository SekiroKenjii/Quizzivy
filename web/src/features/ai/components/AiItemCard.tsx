import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { Check, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AiCard, AiCardAction, AiTone } from "../kind";

const TONES: Readonly<Record<AiTone, string>> = {
  muted: "bg-muted text-muted-fg",
  accent: "bg-brand-soft text-brand-ink",
  danger: "bg-danger-soft text-danger-ink",
  warning: "bg-warning-soft text-warning-ink",
  info: "bg-info-soft text-info-ink",
  success: "bg-success-soft text-success-ink",
};

const LETTERS = "ABCDEFGH";

const ACTION =
  "text-meta inline-flex h-7.5 items-center gap-1.5 rounded-seg border px-2.5 font-medium whitespace-nowrap hover:opacity-90 disabled:cursor-default disabled:opacity-50";

function actionClass(action: AiCardAction) {
  return cn(
    ACTION,
    action.primary
      ? "border-primary bg-primary text-primary-fg"
      : "border-border bg-card text-fg",
  );
}

/**
 * AiItemCard is one suggestion of an AI panel as the deck draws it: a "Keep
 * this" checkbox when it can be kept, its pills and meta, its title and body,
 * lettered options with the key marked, an answer, a before and after, tag
 * chips, a note and its actions. `busy` disables the actions while one runs.
 */
export function AiItemCard({
  card,
  selected,
  busy,
  onToggle,
  onAction,
}: Readonly<{
  card: AiCard;
  selected: boolean;
  busy: boolean;
  onToggle: () => void;
  onAction: (action: AiCardAction) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div
      data-slot="ai-item"
      className={cn(
        "bg-card flex gap-3 rounded-xl border p-3.5",
        card.selectable && selected ? "border-ring" : "border-border",
      )}
    >
      {card.selectable && (
        <button
          type="button"
          role="checkbox"
          aria-checked={selected}
          aria-label={t("ai.keep")}
          onClick={onToggle}
          className={cn(
            "text-primary-fg mt-0.5 grid size-4.5 flex-none place-items-center rounded-[5px] border p-0",
            selected ? "border-primary bg-primary" : "border-ring bg-card",
          )}
        >
          {selected && <Check aria-hidden="true" className="size-3" />}
        </button>
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-2.5">
        <div className="flex flex-wrap items-center gap-1.5">
          {card.tags.map((tag) => (
            <span
              key={tag.label}
              className={cn(
                "inline-flex h-5.5 items-center rounded-full px-2 text-xs font-medium whitespace-nowrap",
                TONES[tag.tone],
              )}
            >
              {tag.label}
            </span>
          ))}
          {card.meta !== undefined && (
            <span className="text-muted-fg ml-auto text-xs">{card.meta}</span>
          )}
        </div>
        <p className="text-base leading-normal font-medium text-pretty break-words">
          {card.title}
        </p>
        {card.body !== undefined && (
          <p className="text-muted-fg text-sm leading-[1.55] text-pretty break-words">
            {card.body}
          </p>
        )}
        {card.options !== undefined && card.options.length > 0 && (
          <ul className="flex flex-col gap-1">
            {card.options.map((option, index) => (
              <li
                key={index}
                className={cn(
                  "rounded-seg flex items-center gap-2 px-2.5 py-1.5 text-sm",
                  option.correct
                    ? "bg-success-soft text-success-ink"
                    : "bg-muted text-fg",
                )}
              >
                <span
                  aria-hidden="true"
                  className="w-3.5 flex-none text-xs font-semibold"
                >
                  {LETTERS[index]}
                </span>
                <span className="min-w-0 flex-1 break-words">{option.text}</span>
                {option.correct && (
                  <>
                    <Check aria-hidden="true" className="size-3.5 flex-none" />
                    <span className="sr-only">{t("ai.correct")}</span>
                  </>
                )}
              </li>
            ))}
          </ul>
        )}
        {card.answer !== undefined && (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-fg">{t("ai.answer")}</span>
            <span className="bg-success-soft text-success-ink rounded-[0.375rem] px-2 py-0.5 font-medium">
              {card.answer}
            </span>
          </div>
        )}
        {card.diff !== undefined && (
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-sm leading-normal">
            <dt className="text-muted-fg">{card.diff.beforeLabel}</dt>
            <dd
              className={cn(
                "text-muted-fg break-words",
                card.diff.strike && "line-through",
              )}
            >
              {card.diff.before}
            </dd>
            <dt className="text-muted-fg">{card.diff.afterLabel}</dt>
            <dd className="break-words">{card.diff.after}</dd>
          </dl>
        )}
        {card.chips !== undefined && card.chips.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {card.chips.map((chip) => (
              <span
                key={chip}
                className="border-ring inline-flex h-5.5 items-center gap-1 rounded-full border border-dashed px-2 text-xs font-medium whitespace-nowrap"
              >
                <Plus aria-hidden="true" className="size-2.75" />
                {chip}
              </span>
            ))}
          </div>
        )}
        {card.note !== undefined && (
          <p className="bg-muted text-meta rounded-[0.5rem] px-2.5 py-2 leading-[1.55] text-pretty">
            <span className="font-semibold">{card.note.label}</span> {card.note.text}
          </p>
        )}
        {card.actions !== undefined && card.actions.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {card.actions.map((action) =>
              action.href === undefined ? (
                <button
                  key={action.label}
                  type="button"
                  disabled={busy}
                  className={actionClass(action)}
                  onClick={() => onAction(action)}
                >
                  {action.label}
                </button>
              ) : (
                <Link
                  key={action.label}
                  to={action.href}
                  className={actionClass(action)}
                >
                  {action.label}
                </Link>
              ),
            )}
          </div>
        )}
      </div>
    </div>
  );
}
