import { useTranslation } from "react-i18next";
import { StatusBadge } from "@/components/shared/StatusBadge";
import type { Assignment } from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import { targetsOf, windowOf, type ListTab } from "./assignmentWindow";

/**
 * AssignmentCell is the Assignment column: the title, "Test v{n} · {q}
 * questions", what is left to grade in warning ink, and `inline`, the
 * "classes · when" line, when the table has hidden those columns.
 */
export function AssignmentCell({
  assignment,
  inline,
}: Readonly<{ assignment: Assignment; inline: string | null }>) {
  const { t } = useTranslation();
  const toGrade = assignment.pendingGradingCount ?? 0;
  return (
    <span className="flex min-w-0 flex-col">
      <span className="truncate font-medium">{assignment.testTitle}</span>
      <span className="text-muted-fg flex flex-wrap gap-x-2 text-[12.5px]">
        <span>
          {t("assignments.list.meta", {
            version: assignment.testVersion,
            count: assignment.questionCount ?? 0,
          })}
        </span>
        {toGrade > 0 ? (
          <span className="text-warning-ink font-medium">
            {t("assignments.list.toGrade", { count: toGrade })}
          </span>
        ) : null}
      </span>
      {inline === null ? null : (
        <span className="text-muted-fg truncate text-[12.5px]">{inline}</span>
      )}
    </span>
  );
}

/** SubmittedBar is the deck's submission bar in the accent, with "x/y" beside it. */
export function SubmittedBar({ assignment }: Readonly<{ assignment: Assignment }>) {
  const { t } = useTranslation();
  const submitted = assignment.submittedCount ?? 0;
  const target = assignment.targetCount ?? 0;
  const percent =
    target === 0 ? 0 : Math.min(100, Math.round((submitted / target) * 100));
  return (
    <span
      className="flex w-full items-center gap-2.5"
      role="img"
      aria-label={t("assignments.list.submittedOf", { submitted, target })}
    >
      <span className="bg-muted h-1.5 flex-1 overflow-hidden rounded-full">
        <span className="bg-brand block h-full" style={{ width: `${percent}%` }} />
      </span>
      <span className="text-muted-fg w-10 text-right text-[12.5px] tabular-nums">
        {submitted}/{target}
      </span>
    </span>
  );
}

/** AssignmentCard is a row below 768px: title and status, "classes · when", then the bar. */
export function AssignmentCard({
  assignment,
  tab,
  now,
}: Readonly<{ assignment: Assignment; tab: ListTab; now: Date }>) {
  const { t } = useTranslation();
  return (
    <>
      <span className="flex items-start justify-between gap-2.5">
        <span className="text-sm font-medium">{assignment.testTitle}</span>
        <StatusBadge
          kind="assignment"
          status={statusAt(assignment, now)}
          className="flex-none"
        />
      </span>
      <span className="text-muted-fg text-[12.5px]">
        {targetsOf(assignment, t)} · {windowOf(assignment, tab, now, t)}
      </span>
      <SubmittedBar assignment={assignment} />
    </>
  );
}
