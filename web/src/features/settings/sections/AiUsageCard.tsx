import { useTranslation } from "react-i18next";
import type { UseQueryResult } from "@tanstack/react-query";
import {
  FileUp,
  MessageSquareText,
  ScanSearch,
  Sparkles,
  SquarePen,
  type LucideIcon,
} from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { QueryStates } from "@/components/shared/ListState";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { Skeleton } from "@/components/ui/skeleton";
import type { AiCreditCategory, AiMonthUsage } from "@/features/settings/aiApi";
import { credits, resetsOn } from "@/features/settings/aiCredits";
import { AiCreditRequest } from "@/features/settings/sections/AiCreditRequest";
import { useLocale } from "@/lib/i18n/useLocale";

const ICONS: Record<AiCreditCategory, LucideIcon> = {
  generation: Sparkles,
  grading: SquarePen,
  explanations: MessageSquareText,
  imports: FileUp,
  checks: ScanSearch,
};

function UsageSkeleton() {
  const { t } = useTranslation();
  return (
    <div role="status" aria-label={t("common.loading")} className="flex flex-col gap-4">
      <Skeleton className="h-7 w-56" />
      <Skeleton className="h-2 w-full" />
      <Skeleton className="h-50 w-full rounded-[10px]" />
    </div>
  );
}

function Limit({ usage }: Readonly<{ usage: AiMonthUsage }>) {
  const { t } = useTranslation();
  if (usage.paused)
    return <Callout tone="warning">{t("settings.ai.usage.paused")}</Callout>;
  if (usage.left > 0) return null;
  return (
    <Callout tone="warning">
      {usage.used >= usage.allowance
        ? t("settings.ai.usage.allowanceUsed")
        : t("settings.ai.usage.poolUsed")}
    </Callout>
  );
}

function Usage({ usage }: Readonly<{ usage: AiMonthUsage }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const used = credits(usage.used, locale);
  const limit = credits(usage.allowance, locale);
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-baseline justify-between gap-2.5">
          <span className="text-stat leading-normal font-semibold tracking-[-0.02em] tabular-nums">
            {used}{" "}
            <span className="text-muted-fg text-sm font-medium tracking-normal">
              {t("settings.ai.usage.of", { limit })}
            </span>
          </span>
          <span className="text-muted-fg text-meta leading-normal tabular-nums">
            {t("settings.ai.usage.left", { left: credits(usage.left, locale) })}
          </span>
        </div>
        <ProgressBar
          value={usage.used}
          max={Math.max(usage.allowance, 1)}
          size={8}
          cap="round"
          label={t("settings.ai.usage.bar", { used, limit })}
        />
      </div>
      <Limit usage={usage} />
      <ul
        aria-label={t("settings.ai.usage.byFeature")}
        className="flex flex-col overflow-hidden rounded-[10px] border"
      >
        {usage.categories.map(({ category, credits: spent }) => {
          const Icon = ICONS[category];
          return (
            <li
              key={category}
              className="flex items-center gap-3 border-t px-3 py-2.5 text-sm first:border-t-0"
            >
              <Icon
                aria-hidden="true"
                className="text-muted-fg size-[15px] flex-none"
              />
              <span className="min-w-0 flex-1">
                {t(`settings.ai.usage.categories.${category}`)}
              </span>
              <span className="text-muted-fg tabular-nums">
                {credits(spent, locale)}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * AiUsageCard is the "AI usage this month" card of Settings › AI assistant:
 * the credits used of the allowance, what is left, the bar and the five
 * category rows of `GET /teacher/ai/usage`, the date the allowance resets,
 * a note while the teacher is paused or out of credits, and the request for
 * more credits under it (DG-200).
 */
export function AiUsageCard({
  query,
}: Readonly<{ query: UseQueryResult<AiMonthUsage> }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const month = query.data?.month;
  return (
    <SettingsCard
      title={t("settings.ai.usage.title")}
      description={
        month === undefined
          ? t("settings.ai.usage.hint")
          : t("settings.ai.usage.hintReset", { date: resetsOn(month, locale) })
      }
    >
      <div className="p-4.5">
        <QueryStates
          query={query}
          skeleton={<UsageSkeleton />}
          failed={t("settings.ai.usage.loadFailed")}
        >
          {(usage) => <Usage usage={usage} />}
        </QueryStates>
      </div>
      {query.data === undefined ? null : <AiCreditRequest usage={query.data} />}
    </SettingsCard>
  );
}
