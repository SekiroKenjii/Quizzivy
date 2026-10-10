import { Avatar } from "@/components/ui/avatar";
import { useTranslation } from "react-i18next";
import type { GradingQueue, GradingQueueItem } from "../../api";
import { gradingGroupKey, gradingItemKey } from "./gradingRecovery";
import { cn } from "@/lib/utils";

/** GradingQueueAside lists the groups in the order the queue first showed their answers, with the server's complete remaining counts and "Done" for a group graded in this session. */
export function GradingQueueAside({
  queue,
  items,
  selected,
  mode,
  busy,
  onSelect,
}: Readonly<{
  queue: GradingQueue | undefined;
  items: GradingQueueItem[];
  selected: GradingQueueItem | undefined;
  mode: "student" | "question";
  busy: boolean;
  onSelect: (item: GradingQueueItem) => void;
}>) {
  const { t } = useTranslation();
  const known = new Map((queue?.groups ?? []).map((group) => [group.key, group]));
  const groups = new Map<string, NonNullable<typeof queue>["groups"][number]>();
  for (const item of items) {
    const key = gradingGroupKey(item, mode);
    if (!groups.has(key))
      groups.set(
        key,
        known.get(key) ?? {
          key,
          kind: mode,
          label: mode === "student" ? item.studentName : String(item.questionNumber),
          sub: item.assignmentTitle,
          remaining: 0,
        },
      );
  }
  return (
    <aside className="bg-card shadow-card w-full min-w-0 overflow-hidden rounded-xl border min-[768px]:max-w-75 min-[768px]:flex-[1_1_260px]">
      <h2 className="text-muted-foreground border-b px-3.5 py-3 text-[12.5px] leading-[1.5] font-medium">
        {t(mode === "student" ? "grading.studentsWaiting" : "grading.questionsWaiting")}
      </h2>
      <div className="max-h-45 overflow-y-auto min-[768px]:max-h-140">
        {[...groups.values()].map((group) => {
          const item =
            items.find(
              (row) =>
                gradingGroupKey(row, mode) === group.key &&
                queue?.items.some(
                  (pending) => gradingItemKey(pending) === gradingItemKey(row),
                ),
            ) ?? items.find((row) => gradingGroupKey(row, mode) === group.key);
          const active = selected && gradingGroupKey(selected, mode) === group.key;
          const label =
            mode === "student"
              ? group.label
              : t("grading.question", { n: group.label });
          return (
            <button
              key={group.key}
              type="button"
              disabled={busy || !item}
              onClick={() => {
                if (item) onSelect(item);
              }}
              aria-label={t("grading.queueRow", {
                name: label,
                assignment: group.sub,
                remaining:
                  group.remaining > 0
                    ? t("grading.left", { count: group.remaining })
                    : t("grading.done"),
              })}
              aria-current={active ? "true" : undefined}
              className={cn(
                "hover:bg-muted flex w-full items-center gap-2.5 border-b px-3.5 py-2.5 text-left leading-[normal] disabled:opacity-50",
                active && "bg-muted shadow-[inset_2px_0_0_var(--accent-c)]",
              )}
            >
              {mode === "student" ? (
                <Avatar name={label} size="30" />
              ) : (
                <span
                  aria-hidden="true"
                  className="bg-muted grid size-7.5 shrink-0 place-items-center rounded-[8px] text-[11px] font-semibold"
                >
                  {t("grading.questionTile", { n: group.label })}
                </span>
              )}
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13.5px] leading-[normal] font-medium">
                  {label}
                </span>
                <span className="text-muted-foreground block truncate text-xs leading-[normal]">
                  {group.sub}
                </span>
              </span>
              <span
                className={cn(
                  "text-xs leading-[normal] font-medium tabular-nums",
                  group.remaining === 0 && "text-success-ink",
                )}
              >
                {group.remaining > 0
                  ? t("grading.left", { count: group.remaining })
                  : t("grading.done")}
              </span>
            </button>
          );
        })}
      </div>
    </aside>
  );
}
