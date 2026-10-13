import { useTranslation } from "react-i18next";
import { Flag } from "lucide-react";
import { Tooltip } from "@/components/shared/Tooltip";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  PROGRESS_GROUPS,
  type ProgressGroup,
} from "@/features/assignments/pages/teacher/assignmentDetail";
import { FLAGGED } from "@/features/integrity/tones";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

/** ProgressFigures are the whole-assignment counts the progress card draws, with the rounded graded average or null. */
export type ProgressFigures = Readonly<
  Record<ProgressGroup, number> & { total: number; average: number | null }
>;

/**
 * ProgressCardProps supplies the card's read state and, once ready, its figures,
 * the roster group the URL selects, and the handler a legend press calls with
 * the group to show or null for everyone.
 */
export type ProgressCardProps = Readonly<
  | { state: "pending" }
  | { state: "error"; onRetry: () => void }
  | {
      state: "ready";
      figures: ProgressFigures;
      selected: ProgressGroup | null;
      onSelect: (group: ProgressGroup | null) => void;
    }
>;

const FRAME =
  "bg-card shadow-card flex flex-wrap items-center gap-x-4 gap-y-2.5 rounded-xl border px-4 py-3";

const DOTS: Readonly<Record<Exclude<ProgressGroup, "flagged">, string>> = {
  submitted: "bg-success",
  inProgress: "bg-info",
  notStarted: "bg-border",
};

const BAR: readonly { group: "submitted" | "inProgress"; fill: string }[] = [
  { group: "submitted", fill: "bg-success" },
  { group: "inProgress", fill: "bg-info" },
];

/** ProgressCard is the assignment's submission progress: a sentence, a two-segment bar, a legend that filters the roster, and the average. */
export function ProgressCard(props: ProgressCardProps) {
  const { t } = useTranslation();
  if (props.state === "pending")
    return (
      <div
        role="status"
        aria-label={t("common.loading")}
        data-slot="progress-card"
        className={cn(FRAME, "min-h-13.5")}
      >
        <Skeleton className="h-4 w-36" />
        <Skeleton className="h-2 min-w-30 flex-[1_1_160px] rounded-[0.5rem]" />
      </div>
    );
  if (props.state === "error")
    return (
      <div data-slot="progress-card" className={FRAME}>
        <p className="text-muted-fg text-ui">
          {t("assignmentDetail.progress.loadFailed")}
        </p>
        <Button variant="outline" size="sm" onClick={props.onRetry}>
          {t("common.retry")}
        </Button>
      </div>
    );
  return <ReadyCard {...props} />;
}

function ReadyCard({
  figures,
  selected,
  onSelect,
}: Readonly<{
  figures: ProgressFigures;
  selected: ProgressGroup | null;
  onSelect: (group: ProgressGroup | null) => void;
}>) {
  const { t } = useTranslation();
  const { total } = figures;
  return (
    <div data-slot="progress-card" className={FRAME}>
      <span className="text-ui leading-normal font-semibold whitespace-nowrap tabular-nums">
        {progressSentence(figures, t)}
      </span>
      <span
        aria-hidden="true"
        data-slot="progress-bar"
        className="bg-muted flex h-2 min-w-30 flex-[1_1_160px] overflow-hidden rounded-[0.5rem]"
      >
        {BAR.map(({ group, fill }) => (
          <span
            key={group}
            data-group={group}
            className={cn(
              "h-full transition-[width] duration-200 motion-reduce:transition-none",
              fill,
            )}
            style={{ width: segmentWidth(figures[group], total) }}
          />
        ))}
      </span>
      <span
        role="group"
        aria-label={t("assignmentDetail.progress.filterLabel")}
        className="flex flex-wrap items-center gap-0.5"
      >
        {PROGRESS_GROUPS.filter(
          (group) => figures[group] > 0 || selected === group,
        ).map((group) => (
          <LegendButton
            key={group}
            group={group}
            count={figures[group]}
            pressed={selected === group}
            onPress={() => onSelect(selected === group ? null : group)}
          />
        ))}
      </span>
      <span className="text-muted-fg text-meta flex items-baseline gap-1.5 border-l pl-3.5 whitespace-nowrap">
        {t("assignmentDetail.progress.average")}
        <span className="text-fg text-md font-semibold tabular-nums">
          {figures.average === null ? "—" : `${figures.average}%`}
        </span>
      </span>
    </div>
  );
}

function LegendButton({
  group,
  count,
  pressed,
  onPress,
}: Readonly<{
  group: ProgressGroup;
  count: number;
  pressed: boolean;
  onPress: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const label = t(`assignmentDetail.progress.groups.${group}`);
  const hint = pressed
    ? t("assignmentDetail.progress.showEveryone")
    : t("assignmentDetail.progress.showOnly", {
        group: label.toLocaleLowerCase(locale),
      });
  return (
    <Tooltip label={hint}>
      <button
        type="button"
        aria-pressed={pressed}
        onClick={onPress}
        className="text-muted-fg text-meta hover:bg-muted aria-pressed:border-border aria-pressed:bg-muted rounded-seg inline-flex h-7 cursor-pointer items-center gap-1.5 border border-transparent px-2.25 leading-[normal] whitespace-nowrap"
      >
        {group === "flagged" ? (
          <Flag aria-hidden="true" className={cn("size-3", FLAGGED.ink)} />
        ) : (
          <span
            aria-hidden="true"
            className={cn("size-2 flex-none rounded-full", DOTS[group])}
          />
        )}
        {label}{" "}
        <span
          className={cn(
            "font-semibold tabular-nums",
            group === "flagged" ? FLAGGED.ink : "text-fg",
          )}
        >
          {count}
        </span>
      </button>
    </Tooltip>
  );
}

function progressSentence(
  figures: ProgressFigures,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  if (figures.total === 0) return t("assignmentDetail.progress.notAssigned");
  if (figures.submitted === figures.total)
    return t("assignmentDetail.progress.allSubmitted", { count: figures.total });
  return t("assignmentDetail.progress.submittedOf", {
    submitted: figures.submitted,
    total: figures.total,
  });
}

function segmentWidth(count: number, total: number): string {
  return `${(total === 0 ? 0 : (count / total) * 100).toFixed(2)}%`;
}
