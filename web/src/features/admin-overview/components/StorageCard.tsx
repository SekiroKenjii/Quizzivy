import { useTranslation } from "react-i18next";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Meter } from "@/components/shared/stats/Meter";
import { credits } from "@/features/ai/credits";
import { formatStorage } from "@/features/media/format";
import type { Usage } from "@/features/organization/api";
import { useLocale } from "@/lib/i18n/useLocale";

type Gauge = Readonly<{
  key: string;
  label: string;
  value: string;
  used: number;
  limit: number | null;
  tone: "accent" | "info" | "warning";
}>;

function GaugeRow({ gauge }: Readonly<{ gauge: Gauge }>) {
  const { label, value, used, limit, tone } = gauge;
  return (
    <div className="flex flex-col gap-1.5">
      <div className="text-ui flex flex-wrap justify-between gap-x-3">
        <span>{label}</span>
        <span className="text-muted-fg tabular-nums">{value}</span>
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

/**
 * StorageCard is the Overview's use of the plan: media, import documents and
 * seats against their limits when a limit is set, the attempts kept as a
 * figure, and the month's AI credits when the server reports them.
 */
export function StorageCard({ usage }: Readonly<{ usage: Usage }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const count = (value: number) => new Intl.NumberFormat(locale).format(value);
  const { media, imports, seats, attemptsKept, aiCredits } = usage;
  const of = (used: string, limit: string | null) =>
    limit === null ? used : t("adminOverview.storage.of", { used, limit });
  const gauges: Gauge[] = [
    {
      key: "media",
      label: t("adminOverview.storage.media"),
      value: of(
        formatStorage(media.bytes),
        media.limit === null ? null : formatStorage(media.limit),
      ),
      used: media.bytes,
      limit: media.limit,
      tone: "accent",
    },
    {
      key: "imports",
      label: t("adminOverview.storage.imports"),
      value: of(
        formatStorage(imports.bytes),
        imports.limit === null ? null : formatStorage(imports.limit),
      ),
      used: imports.bytes,
      limit: imports.limit,
      tone: "warning",
    },
    {
      key: "seats",
      label: t("adminOverview.storage.seats"),
      value: of(count(seats.used), seats.limit === null ? null : count(seats.limit)),
      used: seats.used,
      limit: seats.limit,
      tone: "info",
    },
    {
      key: "attempts",
      label: t("adminOverview.storage.attempts"),
      value: t("adminOverview.storage.attemptsCount", {
        count: attemptsKept.count,
        value: count(attemptsKept.count),
      }),
      used: attemptsKept.count,
      limit: null,
      tone: "info",
    },
  ];
  if (aiCredits !== null) {
    gauges.push({
      key: "credits",
      label: t("adminOverview.storage.credits"),
      value: t("adminOverview.storage.creditsOf", {
        used: credits(aiCredits.used, locale),
        pool: credits(aiCredits.pool, locale),
      }),
      used: aiCredits.used,
      limit: aiCredits.pool,
      tone: "accent",
    });
  }
  return (
    <SettingsCard
      title={t("adminOverview.storage.title")}
      headerClassName="border-b-0 pb-2.5"
      className="min-w-0 flex-[1_1_18.75rem]"
    >
      <div className="flex flex-col gap-3.5 px-4.5 pb-4">
        {gauges.map((gauge) => (
          <GaugeRow key={gauge.key} gauge={gauge} />
        ))}
      </div>
    </SettingsCard>
  );
}
