import { useId } from "react";
import { useTranslation } from "react-i18next";
import { percent } from "@/components/shared/stats/progress";
import type { AiAdminCredits } from "@/features/ai-credits/api";
import {
  lastDayOf,
  monthName,
  paceFigure,
  poolTone,
  projectionAt,
  type BarTone,
} from "@/features/ai-credits/figures";
import { credits, resetsOn } from "@/features/ai/credits";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const FILLS: Record<BarTone, string> = {
  accent: "bg-brand",
  warning: "bg-warning",
  danger: "bg-danger",
};

function PoolBar({
  used,
  total,
  onPace,
  label,
  projected,
}: Readonly<{
  used: number;
  total: number;
  onPace: number;
  label: string;
  projected: string;
}>) {
  const tick = projectionAt(onPace, total);
  const share = total > 0 ? Math.min(100, (used / total) * 100) : 0;
  return (
    <span
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={percent(used, total)}
      className="bg-muted relative block h-2.5 overflow-hidden rounded-[0.625rem]"
    >
      <span
        data-slot="pool-fill"
        className={cn(
          "block h-full rounded-[0.625rem] transition-[width] duration-300 motion-reduce:transition-none",
          FILLS[poolTone(used, total)],
        )}
        style={{ width: `${share}%` }}
      />
      {tick === null ? null : (
        <span
          aria-hidden="true"
          data-slot="pool-projection"
          title={projected}
          className="bg-fg absolute inset-y-0 w-0.5 opacity-35"
          style={{ left: `${tick}%` }}
        />
      )}
    </span>
  );
}

function Fact({
  label,
  value,
  hint,
  warn = false,
}: Readonly<{ label: string; value: string; hint: string; warn?: boolean }>) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <dt className="text-muted-fg text-meta">{label}</dt>
      <dd className="text-md font-semibold tabular-nums">{value}</dd>
      <dd
        className={cn(
          "text-xs leading-normal text-pretty",
          warn ? "text-warning-ink" : "text-muted-fg",
        )}
      >
        {hint}
      </dd>
    </div>
  );
}

/**
 * PoolCard is the centre's month of AI credits: what was used of the pool in
 * 28px, a 10px bar that turns warning from 75% and danger from 90% with a tick
 * where the month is projected to end, and three facts under it. The pool is
 * the plan and what an Admin added; the projection is the server's `onPace`.
 */
export function PoolCard({ view }: Readonly<{ view: AiAdminCredits }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const id = useId();
  const { pool } = view;
  const total = credits(pool.total, locale);
  const used = credits(pool.used, locale);
  const reset = resetsOn(view.month, locale);
  const last = lastDayOf(view.month, locale);
  const note =
    pool.added > 0
      ? t("aiCredits.pool.planAdded", {
          plan: credits(pool.plan, locale),
          added: credits(pool.added, locale),
          date: reset,
        })
      : t("aiCredits.pool.plan", { plan: credits(pool.plan, locale), date: reset });
  return (
    <section
      aria-labelledby={id}
      className="bg-card shadow-card flex flex-col gap-3.5 rounded-xl border px-5 py-4.5"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2.5">
        <h2 id={id} className="text-md leading-normal font-semibold">
          {t("aiCredits.pool.title", { month: monthName(view.month, locale) })}
        </h2>
        <span className="text-muted-fg text-meta">{note}</span>
      </div>
      <div className="flex flex-wrap items-baseline gap-2">
        <span className="text-kpi-sm leading-[1.2] font-semibold tracking-[-0.02em] tabular-nums">
          {used}
        </span>
        <span className="text-muted-fg text-base tabular-nums">
          {t("aiCredits.pool.of", { pool: total })}
        </span>
      </div>
      <PoolBar
        used={pool.used}
        total={pool.total}
        onPace={view.onPace}
        label={t("aiCredits.pool.bar", { used, pool: total })}
        projected={t("aiCredits.pool.projected", { date: last })}
      />
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(11.25rem,1fr))] gap-3.5">
        <Fact
          label={t("aiCredits.pool.left")}
          value={credits(pool.left, locale)}
          hint={
            view.daysToGo > 0
              ? t("aiCredits.pool.daysToGo", { count: view.daysToGo })
              : t("aiCredits.pool.lastDay")
          }
        />
        <Fact
          label={t("aiCredits.pool.pace")}
          value={`~${credits(paceFigure(view.onPace), locale)}`}
          hint={
            view.withinPool
              ? t("aiCredits.pool.paceWithin", { date: last })
              : t("aiCredits.pool.paceOver")
          }
          warn={!view.withinPool}
        />
        <Fact
          label={t("aiCredits.pool.allowances")}
          value={credits(view.allowances, locale)}
          hint={t("aiCredits.pool.allowancesHint")}
        />
      </dl>
    </section>
  );
}
