import { useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { listMyAssignments } from "@/features/assignments/api";
import { nextByClass, type ComingUpRow } from "@/features/assignments/studentHome";
import { JoinDialog } from "@/features/join/components/JoinDialog";
import { useMinute } from "@/hooks/useTick";
import type { Locale } from "@/lib/i18n";
import { appDaysUntil, dayDate, formatTime } from "@/lib/i18n/datetime";
import { myClassesQuery, type MyClass } from "../api";

const CARD = "bg-card shadow-card flex flex-col overflow-hidden rounded-2xl border";
const GRID = "grid grid-cols-[repeat(auto-fill,minmax(min(280px,100%),1fr))] gap-3.5";
const SHAPES = ["first", "second"] as const;

type Next = Readonly<{ kind: "line" | "none" | "loading" | "unknown"; text: string }>;

function nextLine(next: ComingUpRow, now: Date, locale: Locale, t: TFunction): string {
  const title = next.card.testTitle;
  const today = appDaysUntil(next.moment, now) <= 0;
  const time = formatTime(next.moment);
  const date = dayDate(next.moment, locale);
  if (next.pill === "inProgress") return t("student.classes.nextLive", { title });
  if (next.pill === "opens")
    return today
      ? t("student.classes.nextOpensToday", { title, time })
      : t("student.classes.nextOpens", { title, date });
  return today
    ? t("student.classes.nextClosesToday", { title, time })
    : t("student.classes.nextDue", { title, date });
}

/**
 * StudentClassesPage is the student's Classes screen, as the design deck
 * draws it: the classes as cards, each with its teacher and the paper that
 * comes next there, and "Join a class", which opens the Join dialog. A card is
 * not a link, and nothing here leaves a class. "Next" follows Home's order and
 * repaints each minute. Until the timetable ships, the line under a class's
 * name is its description. A list that has loaded stays on screen when a
 * later refetch fails.
 */
export default function StudentClassesPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const [joining, setJoining] = useState(false);
  const classes = useQuery(myClassesQuery);
  const assignments = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
  });
  useMinute(true);
  const now = new Date();
  const next =
    assignments.data === undefined ? null : nextByClass(assignments.data, now);
  const nextFor = (classId: string): Next => {
    if (next === null)
      return { kind: assignments.isPending ? "loading" : "unknown", text: "" };
    const row = next.get(classId);
    return row === undefined
      ? { kind: "none", text: "" }
      : { kind: "line", text: nextLine(row, now, locale, t) };
  };

  return (
    <div className="mx-auto flex w-full max-w-240 flex-col gap-4.5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-stat min-[768px]:text-h1-student font-semibold tracking-[-0.02em]">
            {t("student.myClasses")}
          </h1>
          <p className="text-muted-fg mt-0.5 text-base">
            {t("student.classes.subtitle")}
          </p>
        </div>
        <Button
          size="md"
          className="h-10 min-h-0 gap-1.5 px-4 text-base"
          onClick={() => setJoining(true)}
        >
          <Plus aria-hidden="true" className="size-4" />
          {t("student.joinClass")}
        </Button>
      </div>
      {classes.data === undefined && classes.isPending && (
        <div role="status" aria-live="polite" aria-label={t("common.loading")}>
          <div className={GRID}>
            {SHAPES.map((shape) => (
              <div key={shape} className={CARD}>
                <div className="flex flex-col gap-3.5 p-4.5">
                  <Skeleton className="h-5 w-[60%] rounded-sm" />
                  <div className="flex items-center gap-2.5">
                    <Skeleton className="size-8.5 rounded-full" />
                    <Skeleton className="h-3.5 w-[40%] rounded-sm" />
                  </div>
                </div>
                <div className="bg-sidebar flex flex-col gap-2 border-t px-4.5 py-3">
                  <Skeleton className="h-3 w-12 rounded-sm" />
                  <Skeleton className="h-3.5 w-[70%] rounded-sm" />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
      {classes.data === undefined && !classes.isPending && (
        <LoadError error={classes.error} onRetry={() => void classes.refetch()}>
          {t("student.loadFailed")}
        </LoadError>
      )}
      {classes.data?.items.length === 0 && (
        <EmptyState>{t("student.noClasses")}</EmptyState>
      )}
      {classes.data !== undefined && classes.data.items.length > 0 && (
        <div className={GRID}>
          {classes.data.items.map((c) => (
            <ClassCard key={c.id} klass={c} next={nextFor(c.id)} />
          ))}
        </div>
      )}
      <JoinDialog open={joining} onOpenChange={setJoining} />
    </div>
  );
}

function ClassCard({ klass: c, next }: Readonly<{ klass: MyClass; next: Next }>) {
  const { t } = useTranslation();
  return (
    <article className={CARD}>
      <div className="flex flex-col gap-3.5 p-4.5">
        <div className="min-w-0">
          <h2 className="text-title leading-normal font-semibold break-words">
            {c.name}
          </h2>
          {c.description !== null && c.description !== "" && (
            <p className="text-muted-fg text-sm break-words">{c.description}</p>
          )}
        </div>
        {c.teacherName !== null && (
          <div className="flex items-center gap-2.5">
            <Avatar name={c.teacherName} size="36" className="size-8.5 font-semibold" />
            <span className="min-w-0">
              <span className="text-ui block leading-normal font-medium break-words">
                {c.teacherName}
              </span>
              <span className="text-muted-fg text-meta block leading-normal">
                {t("student.classes.teacher")}
              </span>
            </span>
          </div>
        )}
      </div>
      <div className="bg-sidebar mt-auto flex flex-col gap-1 border-t px-4.5 py-3">
        <span className="text-muted-fg text-xs leading-normal">
          {t("student.classes.next")}
        </span>
        {next.kind === "loading" && (
          <Skeleton className="my-1 h-3.5 w-[70%] rounded-sm" />
        )}
        {next.kind === "unknown" && (
          <span className="text-muted-fg text-ui leading-normal">
            {t("student.classes.nextUnknown")}
          </span>
        )}
        {(next.kind === "line" || next.kind === "none") && (
          <span className="text-ui leading-normal font-medium break-words">
            {next.kind === "line" ? next.text : t("student.classes.nextNone")}
          </span>
        )}
      </div>
    </article>
  );
}
