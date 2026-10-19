import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Meter } from "@/components/shared/stats/Meter";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { credits } from "@/features/ai/credits";
import { usageQuery, type Usage } from "@/features/organization/api";
import { formatStorage } from "@/features/media/format";
import { formatDate } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";

type Tone = "accent" | "info" | "warning";

function UsageRow({
  label,
  value,
  note,
  used,
  limit,
  tone,
}: Readonly<{
  label: string;
  value: string;
  note?: string | undefined;
  used: number;
  limit: number | null;
  tone: Tone;
}>) {
  return (
    <div className="flex flex-col gap-1.5 px-4.5 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <span className="text-ui leading-normal font-medium">{label}</span>
        <span className="text-muted-fg text-meta leading-normal">
          {value}
          {note === undefined ? null : ` · ${note}`}
        </span>
      </div>
      {limit === null || limit <= 0 ? null : (
        <Meter
          label={label}
          valueText={value}
          max={limit}
          parts={[{ key: "used", value: used, tone }]}
          className="h-1.5"
        />
      )}
    </div>
  );
}

function ofLimit(used: string, limit: string | null) {
  return limit === null ? used : `${used} / ${limit}`;
}

function Rows({ usage }: Readonly<{ usage: Usage }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const count = (value: number) => new Intl.NumberFormat(locale).format(value);
  const { seats, teachers, media, imports, aiCredits } = usage;
  return (
    <div className="divide-y">
      <UsageRow
        label={t("systemSettings.plan.seats")}
        value={ofLimit(
          count(seats.used),
          seats.limit === null ? null : count(seats.limit),
        )}
        used={seats.used}
        limit={seats.limit}
        tone="info"
      />
      <UsageRow
        label={t("systemSettings.plan.media")}
        value={ofLimit(
          formatStorage(media.bytes),
          media.limit === null ? null : formatStorage(media.limit),
        )}
        used={media.bytes}
        limit={media.limit}
        tone="accent"
      />
      <UsageRow
        label={t("systemSettings.plan.imports")}
        value={ofLimit(
          formatStorage(imports.bytes),
          imports.limit === null ? null : formatStorage(imports.limit),
        )}
        note={t("systemSettings.plan.importsCount", { count: imports.count })}
        used={imports.bytes}
        limit={imports.limit}
        tone="warning"
      />
      <UsageRow
        label={t("systemSettings.plan.teachers")}
        value={ofLimit(
          count(teachers.used),
          teachers.limit === null ? null : count(teachers.limit),
        )}
        used={teachers.used}
        limit={teachers.limit}
        tone="info"
      />
      {aiCredits === null ? null : (
        <UsageRow
          label={t("systemSettings.plan.aiCredits")}
          value={t("systemSettings.plan.aiCreditsOf", {
            used: credits(aiCredits.used, locale),
            pool: credits(aiCredits.pool, locale),
          })}
          used={aiCredits.used}
          limit={aiCredits.pool}
          tone="accent"
        />
      )}
    </div>
  );
}

function UsageBody({ query }: Readonly<{ query: UseQueryResult<Usage> }>) {
  const { t } = useTranslation();
  if (query.data !== undefined) return <Rows usage={query.data} />;
  if (query.isError) {
    return (
      <div className="flex flex-wrap items-center gap-3 px-4.5 py-4">
        <p role="alert" className="text-muted-fg text-ui min-w-40 flex-1">
          {t("systemSettings.plan.loadFailed")}
        </p>
        <Button
          type="button"
          variant="outline"
          className="h-8.5"
          onClick={() => void query.refetch()}
        >
          {t("common.retry")}
        </Button>
      </div>
    );
  }
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="flex flex-col gap-4 px-4.5 py-4"
    >
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-full" />
    </div>
  );
}

function planLine(plan: Usage["plan"], locale: Locale, t: TFunction): string {
  if (plan === null) return t("systemSettings.plan.hint");
  if (plan.renewsOn === null)
    return t("systemSettings.plan.named", { name: plan.name });
  return t("systemSettings.plan.namedRenews", {
    name: plan.name,
    date: formatDate(plan.renewsOn, locale, "UTC"),
  });
}

/**
 * PlanUsageSection is the Plan & usage card of System settings, read-only: the
 * centre's plan and when it renews, when it has one, and a meter per
 * resource, seats, media storage, import document storage and teachers, that
 * shows only its figure when no limit is configured. The AI credits row is
 * there only while the server reports the month's credits.
 */
export function PlanUsageSection() {
  const { t } = useTranslation();
  const locale = useLocale();
  const usage = useQuery(usageQuery());
  return (
    <SettingsCard
      title={t("systemSettings.plan.title")}
      description={planLine(usage.data?.plan ?? null, locale, t)}
    >
      <UsageBody query={usage} />
    </SettingsCard>
  );
}
