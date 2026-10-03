import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Hourglass } from "lucide-react";
import { useTick } from "@/hooks/useTick";
import type { Locale } from "@/lib/i18n";
import { dayDate } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { StudentAssignmentCard } from "../api";
import { justSubmitted, type ResultRow } from "../studentHome";
import { HOME_ITEM, HOME_LIST, HOME_ROW } from "./homeStyles";

function Outcome({ row, locale }: Readonly<{ row: ResultRow; locale: Locale }>) {
  const { t } = useTranslation();
  const score = row.card.score;
  if (row.outcome === "grading")
    return (
      <span className="bg-warning-soft text-warning-ink text-meta inline-flex flex-none items-center gap-[5px] rounded-full px-[9px] py-0.5 leading-4 whitespace-nowrap">
        <Hourglass aria-hidden="true" className="size-3" />
        {t("student.home.beingGraded")}
      </span>
    );
  if (row.outcome === "score" && score != null) {
    const n = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 });
    return (
      <span className="text-title flex-none leading-5 font-semibold whitespace-nowrap tabular-nums">
        {t("student.home.score", {
          earned: n.format(score.earned),
          total: n.format(score.total),
        })}
      </span>
    );
  }
  return (
    <span className="text-muted-fg flex-none text-sm leading-4 whitespace-nowrap">
      {t("student.home.submitted")}
    </span>
  );
}

function when(card: StudentAssignmentCard, now: Date, locale: Locale, fresh: string) {
  if (card.lastSubmittedAt == null) return null;
  return justSubmitted(card, now) ? fresh : dayDate(card.lastSubmittedAt, locale);
}

/**
 * RecentResults lists the papers attempted last, each a link to its result:
 * the title, the day it was submitted, and the score, "being graded" while
 * an answer waits for the teacher, or "submitted" when the score is not
 * shown. The newest reads "just now" for its first minute, and the row
 * repaints by itself when that minute ends.
 */
export function RecentResults({
  results,
}: Readonly<{ results: readonly ResultRow[] }>) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const heading = useId();
  const now = new Date();
  useTick(results.some((row) => justSubmitted(row.card, now)));
  return (
    <section aria-labelledby={heading} className="flex flex-col gap-2.5">
      <h2 id={heading} className="text-title leading-normal font-semibold">
        {t("student.home.recent")}
      </h2>
      <ul className={HOME_LIST}>
        {results.map((row) => {
          const { card } = row;
          const date = when(card, now, locale, t("student.home.justNow"));
          return (
            <li key={card.id} className={HOME_ITEM}>
              <Link
                to={`/app/attempts/${card.lastAttemptId}/result`}
                className={cn(HOME_ROW, "hover:bg-muted")}
              >
                <span className="min-w-0 flex-1">
                  <span className="text-body block leading-[1.35] font-medium break-words">
                    {card.testTitle}
                  </span>
                  {date !== null && (
                    <span className="text-muted-fg block text-sm leading-4">
                      {date}
                    </span>
                  )}
                </span>
                <Outcome row={row} locale={locale} />
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
