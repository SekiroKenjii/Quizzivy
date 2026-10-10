import { useTranslation } from "react-i18next";
import {
  ArrowRight,
  CircleAlert,
  CircleCheck,
  CircleHelp,
  CloudCheck,
  CloudOff,
  LoaderCircle,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import type { AutosaveStatus } from "@/features/tests/useAutosave";
import { cn } from "@/lib/utils";

/** ReviewCounts are the top bar's figures: included questions, those with an answer, and sections. */
export interface ReviewCounts {
  questions: number;
  answers: number;
  sections: number;
}

type Pill = { icon: LucideIcon; label: string; tone: string };

function SaveStatus({
  status,
  staleLabel,
  onRetry,
}: Readonly<{ status: AutosaveStatus; staleLabel: string; onRetry: () => void }>) {
  const { t } = useTranslation();
  const failed = status.kind === "failed";
  const stale = status.kind === "stale";
  const saving = status.kind === "saving" || status.kind === "dirty";
  let Icon: LucideIcon = CloudCheck;
  let label = t("imports.review.saved");
  if (saving) {
    Icon = LoaderCircle;
    label = t("imports.review.saving");
  } else if (failed) {
    Icon = CloudOff;
    label = t("imports.review.saveFailed");
  } else if (stale) {
    Icon = CloudOff;
    label = staleLabel;
  }
  return (
    <>
      <span
        role="status"
        aria-live="polite"
        data-state={status.kind}
        className={cn(
          "inline-flex items-center gap-1.25",
          failed || stale ? "text-danger-ink" : "text-muted-fg",
        )}
      >
        <Icon aria-hidden="true" className="size-3.25 flex-none" />
        {label}
      </span>
      {failed ? (
        <button
          type="button"
          onClick={onRetry}
          className="text-danger-ink cursor-pointer rounded-sm font-semibold underline underline-offset-2"
        >
          {t("imports.review.retrySave")}
        </button>
      ) : null}
    </>
  );
}

/**
 * ReviewTopBar is the head of the review card: the editable test title, the
 * counts and the save status (Saved, Saving…, or a failed save with Retry),
 * the open items by severity, and "Preview & finish".
 */
export function ReviewTopBar({
  title,
  readOnly,
  onTitle,
  counts,
  status,
  staleLabel,
  onRetry,
  blocking,
  review,
  finishDisabled,
  onFinish,
}: Readonly<{
  title: string;
  readOnly: boolean;
  onTitle: (title: string) => void;
  counts: ReviewCounts;
  status: AutosaveStatus;
  staleLabel: string;
  onRetry: () => void;
  blocking: number;
  review: number;
  finishDisabled: boolean;
  onFinish: () => void;
}>) {
  const { t } = useTranslation();
  const pills: Pill[] =
    blocking + review === 0
      ? [
          {
            icon: CircleCheck,
            label: t("imports.review.nothingOpen"),
            tone: "bg-success-soft text-success-ink",
          },
        ]
      : [
          ...(blocking > 0
            ? [
                {
                  icon: CircleAlert,
                  label: t("imports.review.needAction", { count: blocking }),
                  tone: "bg-danger-soft text-danger-ink",
                },
              ]
            : []),
          ...(review > 0
            ? [
                {
                  icon: CircleHelp,
                  label: t("imports.review.toConfirm", { count: review }),
                  tone: "bg-warning-soft text-warning-ink",
                },
              ]
            : []),
        ];
  return (
    <div className="flex flex-none flex-wrap items-center gap-3 border-b px-4 py-2.5">
      <div className="flex min-w-0 flex-[1_1_260px] flex-col">
        <input
          value={title}
          maxLength={200}
          disabled={readOnly}
          aria-label={t("imports.review.titleLabel")}
          onChange={(event) => onTitle(event.target.value)}
          className="text-md hover:border-border -mx-1.5 min-w-0 truncate rounded-md border border-transparent bg-transparent px-1.5 font-semibold disabled:opacity-100"
        />
        <span className="text-muted-fg text-meta flex flex-wrap items-center gap-2">
          <span>
            {t("imports.review.countsLine", {
              questions: t("imports.review.questionsCount", {
                count: counts.questions,
              }),
              answers: t("imports.review.answersCount", { count: counts.answers }),
              sections: t("imports.review.sectionsCount", { count: counts.sections }),
            })}
          </span>
          <span aria-hidden="true">·</span>
          <SaveStatus status={status} staleLabel={staleLabel} onRetry={onRetry} />
        </span>
      </div>
      <ul
        aria-label={t("imports.review.openItems")}
        className="m-0 flex list-none flex-wrap gap-1.5 p-0"
      >
        {pills.map(({ icon: Icon, label, tone }) => (
          <li
            key={label}
            className={cn(
              "text-meta inline-flex h-6.5 items-center gap-1.25 rounded-[7px] px-2.25 font-medium whitespace-nowrap",
              tone,
            )}
          >
            <Icon aria-hidden="true" className="size-3.25 flex-none" />
            {label}
          </li>
        ))}
      </ul>
      <Button
        className="h-9 font-semibold"
        disabled={finishDisabled}
        onClick={onFinish}
      >
        {t("imports.review.finish")}
        <ArrowRight aria-hidden="true" />
      </Button>
    </div>
  );
}
