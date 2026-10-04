import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { Locale } from "@/lib/i18n";
import {
  appDaysUntil,
  dayMonth,
  dayOfMonth,
  formatTime,
  weekdayDate,
  weekdayShort,
} from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { ComingUpPill, ComingUpRow } from "../studentHome";
import { HOME_ITEM, HOME_LIST, HOME_PILL, HOME_ROW } from "./homeStyles";

const TONE = {
  inProgress: { variant: "info", className: "" },
  dueToday: { variant: "warning", className: "" },
  dueTomorrow: { variant: "warning", className: "" },
  open: { variant: "success", className: "" },
  opens: { variant: "secondary", className: "bg-muted text-muted-fg" },
} as const satisfies Record<ComingUpPill, { variant: string; className: string }>;

function opensWhen(moment: string, now: Date, locale: Locale): string {
  const days = appDaysUntil(moment, now);
  if (days <= 0) return formatTime(moment);
  return days < 7 ? weekdayShort(moment, locale) : dayMonth(moment, locale);
}

/**
 * ComingUp lists the papers still to do, each a link to its intro: the date
 * on a tile, the title, the class with the length and the question count, and
 * a pill saying where the paper stands. The tile and the pill turn amber only
 * for a paper closing within 24 hours.
 */
export function ComingUp({
  rows,
  now,
}: Readonly<{ rows: readonly ComingUpRow[]; now: Date }>) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const heading = useId();
  return (
    <section aria-labelledby={heading} className="flex flex-col gap-2.5">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id={heading} className="text-title leading-normal font-semibold">
          {t("student.home.comingUp")}
        </h2>
        <span className="text-muted-fg text-sm">
          {t("student.home.tests", { count: rows.length })}
        </span>
      </div>
      <ul className={HOME_LIST}>
        {rows.map(({ card, pill, moment }) => {
          const urgent = pill === "dueToday" || pill === "dueTomorrow";
          const meta = [
            card.className,
            t("student.home.minutes", { count: card.durationMinutes }),
            t("student.questions", { count: card.questionCount }),
          ]
            .filter((part) => part != null && part !== "")
            .join(" · ");
          return (
            <li key={card.id} className={HOME_ITEM}>
              <Link
                to={`/app/assignments/${card.id}`}
                className={cn(HOME_ROW, "hover:bg-muted")}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    "rounded-ctl flex w-11 flex-none flex-col items-center py-1",
                    urgent ? "bg-warning-soft text-warning-ink" : "bg-muted text-fg",
                  )}
                >
                  <span className="text-3xs leading-3.5 font-semibold tracking-[0.04em] uppercase">
                    {weekdayShort(moment, locale)}
                  </span>
                  <span className="text-lg leading-[1.1] font-semibold">
                    {dayOfMonth(moment)}
                  </span>
                </span>
                <span className="sr-only">{weekdayDate(moment, locale)}</span>
                <span className="min-w-0 flex-1">
                  <span className="text-body block leading-[1.35] font-medium break-words">
                    {card.testTitle}
                  </span>
                  <span className="text-muted-fg block text-sm leading-4 break-words">
                    {meta}
                  </span>
                </span>
                <Badge
                  variant={TONE[pill].variant}
                  className={cn(HOME_PILL, TONE[pill].className)}
                >
                  {pill === "inProgress"
                    ? t("student.home.inProgress")
                    : t(`student.home.pill.${pill}`, {
                        when: opensWhen(moment, now, locale),
                      })}
                </Badge>
                <ChevronRight
                  aria-hidden="true"
                  className="text-muted-fg size-4 flex-none"
                />
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
