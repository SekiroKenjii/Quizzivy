import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { fetchMyClasses } from "@/features/classes/api";
import type { Locale } from "@/lib/i18n";
import {
  appDaysUntil,
  clockTime,
  dayMonth,
  formatTime,
  weekdayName,
} from "@/lib/i18n/datetime";
import { useMinute } from "@/hooks/useTick";
import { useAuthStore } from "@/stores/auth";
import { listMyAssignments } from "../api";
import { ComingUp } from "../components/ComingUp";
import { HomeSkeleton } from "../components/HomeSkeleton";
import { RecentResults } from "../components/RecentResults";
import { ResumeCard } from "../components/ResumeCard";
import { greetingPeriod, homeView, type HomeSub } from "../studentHome";
import { givenName } from "../studentTime";

function subLine(sub: HomeSub, now: Date, locale: Locale, t: TFunction): string | null {
  switch (sub.kind) {
    case "live":
      return t("student.home.subLive", { time: clockTime(sub.closes, now) });
    case "dueToday":
      return t("student.dueToday", { count: sub.count });
    case "next": {
      const days = appDaysUntil(sub.moment, now);
      if (days <= 0)
        return t("student.home.subNextToday", {
          title: sub.title,
          time: formatTime(sub.moment),
        });
      return t("student.home.subNext", {
        title: sub.title,
        when: days < 7 ? weekdayName(sub.moment, locale) : dayMonth(sub.moment, locale),
      });
    }
    case "nothing":
      return t("student.home.subNothing");
    case "none":
      return null;
  }
}

/**
 * StudentHomePage is the student's Home, as the design deck draws it: a
 * greeting for the time of day, one sentence about what is next, the attempt
 * in progress, the papers still to do and the latest results. Everything on
 * it is derived from the assignment lists the shell's badge also reads, and
 * from the clock: the page repaints each minute, so a tab left open does not
 * keep yesterday's greeting and pills. A list that has loaded stays on screen
 * when a later refetch fails.
 */
export default function StudentHomePage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const user = useAuthStore((s) => s.user);
  const assignments = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
  });
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
  });
  useMinute(true);
  const now = new Date();
  const view = assignments.data === undefined ? null : homeView(assignments.data, now);
  const line = view === null ? null : subLine(view.sub, now, locale, t);
  const nothing = view?.sub.kind === "none";

  return (
    <div className="mx-auto flex w-full max-w-240 flex-col gap-6">
      <div>
        <h1 className="text-stat min-[768px]:text-h1-student font-semibold tracking-[-0.02em]">
          {t(`student.home.greeting.${greetingPeriod(now)}`, {
            name: givenName(user?.fullName ?? ""),
          })}
        </h1>
        {line !== null && (
          <p className="text-muted-fg text-body mt-1 leading-normal">{line}</p>
        )}
      </div>
      {view === null && assignments.isPending && <HomeSkeleton />}
      {view === null && !assignments.isPending && (
        <LoadError error={assignments.error} onRetry={() => void assignments.refetch()}>
          {t("student.loadFailed")}
        </LoadError>
      )}
      {nothing && classes.isPending && <HomeSkeleton />}
      {nothing && classes.data?.items.length === 0 && (
        <EmptyState
          action={
            <Button asChild>
              <Link to="/join">{t("student.joinClass")}</Link>
            </Button>
          }
        >
          {t("student.noClasses")}
        </EmptyState>
      )}
      {nothing && !classes.isPending && classes.data?.items.length !== 0 && (
        <EmptyState hint={t("student.noAssignmentsHint")}>
          {t("student.noAssignments")}
        </EmptyState>
      )}
      {view !== null && view.resume !== null && (
        <ResumeCard key={view.resume.id} card={view.resume} />
      )}
      {view !== null && view.rows.length > 0 && <ComingUp rows={view.rows} now={now} />}
      {view !== null && view.results.length > 0 && (
        <RecentResults results={view.results} />
      )}
    </div>
  );
}
