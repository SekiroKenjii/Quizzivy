import { useTranslation } from "react-i18next";
import {
  AlignLeft,
  CircleCheck,
  CircleDot,
  Headphones,
  SquareCheck,
  TextCursorInput,
} from "lucide-react";
import type { AdminQuestion } from "@/features/question-bank/api";
import { formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { promptLine } from "./bankFilters";

const CHIP =
  "bg-muted text-fg inline-flex h-5.5 items-center rounded-full border px-2 text-xs leading-none font-medium whitespace-nowrap";

/** TypeIcon is the deck's icon for a question's type, or headphones for one with audio. */
export function TypeIcon({
  question,
  className,
}: Readonly<{ question: AdminQuestion; className: string }>) {
  if (question.media?.kind === "audio")
    return <Headphones aria-hidden="true" className={className} />;
  switch (question.type) {
    case "single_choice":
      return <CircleDot aria-hidden="true" className={className} />;
    case "multiple_choice":
      return <SquareCheck aria-hidden="true" className={className} />;
    case "true_false":
      return <CircleCheck aria-hidden="true" className={className} />;
    case "fill_blank":
      return <TextCursorInput aria-hidden="true" className={className} />;
    case "short_answer":
      return <AlignLeft aria-hidden="true" className={className} />;
  }
}

/** TypeCell is the Type column: the type's icon and name. */
export function TypeCell({ question }: Readonly<{ question: AdminQuestion }>) {
  const { t } = useTranslation();
  return (
    <span className="flex min-w-0 items-center gap-1.75 text-sm leading-normal">
      <TypeIcon question={question} className="text-muted-fg size-3.5 flex-none" />
      <span className="min-w-0 break-words">
        {t(`questionEditor.type.${question.type}`)}
      </span>
    </span>
  );
}

/**
 * QuestionCell is the Question column: the prompt on one line and, when
 * `inline`, the type and level under it, for a table too narrow for their
 * columns.
 */
export function QuestionCell({
  question,
  inline,
}: Readonly<{ question: AdminQuestion; inline: boolean }>) {
  const { t } = useTranslation();
  const type = t(`questionEditor.type.${question.type}`);
  return (
    <span className="block min-w-0">
      <span className="block truncate">{promptLine(question)}</span>
      {inline && (
        <span className="text-muted-fg mt-0.5 flex items-center gap-1.5 text-xs leading-normal">
          <TypeIcon question={question} className="size-3 flex-none" />
          {question.level === null
            ? type
            : t("bank.typeAndLevel", {
                type,
                level: t(`bank.level.${question.level}`),
              })}
        </span>
      )}
    </span>
  );
}

/** TagChips is a question's tags as the deck's chips, under its prompt. */
export function TagChips({ tags }: Readonly<{ tags: readonly string[] }>) {
  if (tags.length === 0) return null;
  return (
    <span className="mt-0.75 flex flex-wrap gap-1.25">
      {tags.map((tag) => (
        <span key={tag} className={CHIP}>
          {tag}
        </span>
      ))}
    </span>
  );
}

/** LevelCell is the Level column, a dash for a question without a level. */
export function LevelCell({ question }: Readonly<{ question: AdminQuestion }>) {
  const { t } = useTranslation();
  return (
    <span className="block text-sm leading-normal">
      {question.level === null ? "—" : t(`bank.level.${question.level}`)}
    </span>
  );
}

/** UsedCell is the Used column: how many tests use the question. */
export function UsedCell({ question }: Readonly<{ question: AdminQuestion }>) {
  const { t } = useTranslation();
  return (
    <span className="text-muted-fg block text-sm leading-normal tabular-nums">
      {t("bank.usedTests", { count: question.usedInTests ?? 0 })}
    </span>
  );
}

/** UpdatedCell is the Updated column, relative to now. */
export function UpdatedCell({ question }: Readonly<{ question: AdminQuestion }>) {
  const locale = useLocale();
  return (
    <span className="text-muted-fg block text-sm leading-normal">
      {formatRelative(question.updatedAt, locale)}
    </span>
  );
}
