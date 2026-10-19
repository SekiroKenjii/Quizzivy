import type { ReactNode } from "react";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { Trans, useTranslation } from "react-i18next";
import { Activity, GraduationCap, LayoutGrid, Users } from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { BarChart, type BarChartSeries } from "@/components/shared/charts/BarChart";
import { LoadError } from "@/components/shared/ListState";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { KpiTile } from "@/components/shared/stats/KpiTile";
import { Skeleton } from "@/components/ui/skeleton";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { PageHead } from "@/layouts/shell/PageHead";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { useLocale } from "@/lib/i18n/useLocale";
import {
  adminOverviewQuery,
  type AdminDailyActive,
  type AdminOverview,
} from "../../api";
import { AttentionList } from "../../components/AttentionList";
import { RecentChanges } from "../../components/RecentChanges";
import { StorageCard } from "../../components/StorageCard";

const FOUR_COLUMNS_FROM = 900;

type Overview = UseQueryResult<AdminOverview>;

function Failed({ query }: Readonly<{ query: Overview }>) {
  const { t } = useTranslation();
  return (
    <div className="p-4.5 pt-0">
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("adminOverview.loadFailed")}
      </LoadError>
    </div>
  );
}

function Pending() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="flex flex-col gap-3 px-4.5 pb-4.5"
    >
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-4 w-full" />
    </div>
  );
}

function Waiting({ query }: Readonly<{ query: Overview }>) {
  return query.isError ? <Failed query={query} /> : <Pending />;
}

function Kpis({ data }: Readonly<{ data: AdminOverview }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const four = useContentWidthAtLeast(FOUR_COLUMNS_FROM);
  const n = (value: number) => new Intl.NumberFormat(locale).format(value);
  const { people, classes } = data;
  const view = t("adminOverview.kpi.view");
  const tiles: ReactNode[] = [];
  if (people !== null) {
    const percent =
      people.total === 0 ? 0 : Math.round((people.activeThisWeek / people.total) * 100);
    tiles.push(
      <KpiTile
        key="active"
        label={t("adminOverview.kpi.active")}
        icon={Activity}
        tone="muted"
        value={n(people.activeThisWeek)}
        hint={t("adminOverview.kpi.activeHint", {
          percent,
          total: n(people.total),
        })}
        to={ADMIN_PATHS.users}
        action={view}
      />,
      <KpiTile
        key="teachers"
        label={t("adminOverview.kpi.teachers")}
        icon={GraduationCap}
        tone="muted"
        value={n(people.teachers)}
        hint={t("adminOverview.kpi.teachersHint", { count: people.teachersDisabled })}
        to={`${ADMIN_PATHS.users}?tab=teachers`}
        action={view}
      />,
      <KpiTile
        key="students"
        label={t("adminOverview.kpi.students")}
        icon={Users}
        tone="muted"
        value={n(people.students)}
        hint={t("adminOverview.kpi.studentsHint", {
          count: people.studentsNewThisMonth,
        })}
        to={`${ADMIN_PATHS.users}?tab=students`}
        action={view}
      />,
    );
  }
  if (classes !== null) {
    tiles.push(
      <KpiTile
        key="classes"
        label={t("adminOverview.kpi.classes")}
        icon={LayoutGrid}
        tone="muted"
        value={n(classes.active)}
        hint={t("adminOverview.kpi.classesHint", { count: classes.archived })}
        to={ADMIN_PATHS.classes}
        action={view}
      />,
    );
  }
  if (tiles.length === 0) return null;
  return (
    <div
      className="grid gap-3"
      style={{
        gridTemplateColumns: `repeat(${four ? 4 : 2}, minmax(0, 1fr))`,
      }}
    >
      {tiles}
    </div>
  );
}

function KpiSkeleton() {
  const four = useContentWidthAtLeast(FOUR_COLUMNS_FROM);
  return (
    <div
      aria-hidden="true"
      className="grid gap-3"
      style={{ gridTemplateColumns: `repeat(${four ? 4 : 2}, minmax(0, 1fr))` }}
    >
      {[0, 1, 2, 3].map((slot) => (
        <Skeleton key={slot} className="h-28 rounded-xl" />
      ))}
    </div>
  );
}

function DailyActive({ daily }: Readonly<{ daily: AdminDailyActive }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const wide = useMediaQuery("(min-width: 768px)");
  const series: readonly BarChartSeries[] = [
    { key: "students", label: t("adminOverview.daily.students"), tone: "accent" },
    { key: "staff", label: t("adminOverview.daily.teachers"), tone: "info" },
  ];
  return (
    <div className="px-4.5 pb-3.5">
      <p className="text-muted-fg text-sm">
        <Trans
          i18nKey="adminOverview.daily.summary"
          values={{ average: new Intl.NumberFormat(locale).format(daily.average) }}
          components={{ strong: <strong className="text-fg font-semibold" /> }}
        />
      </p>
      <BarChart
        className="mt-4.5"
        caption={t("adminOverview.daily.title")}
        columns={[t("adminOverview.daily.date"), t("adminOverview.daily.total")]}
        series={series}
        labelEvery={wide ? 1 : 2}
        data={daily.days.map((day) => {
          const total = day.students + day.staff;
          return {
            key: day.date,
            label: String(Number(day.date.slice(-2))),
            value: total,
            parts: [day.students, day.staff],
            title: t("adminOverview.daily.tip", {
              total,
              students: day.students,
              staff: day.staff,
              date: day.date.split("-").reverse().join("/"),
            }),
          };
        })}
      />
    </div>
  );
}

/**
 * OverviewPage is the Admin console's home: the centre's headline figures,
 * who was active on each of the last fourteen days, what needs doing, the
 * newest audit entries and what the plan is used for. A section the caller's
 * permissions do not open arrives as null and is left out; each card has its
 * own loading and error state.
 */
export default function OverviewPage() {
  const { t } = useTranslation();
  const query = useQuery(adminOverviewQuery());
  const data = query.data;
  const locale = useLocale();
  const people = data?.people ?? null;
  let description: string | undefined;
  if (data !== undefined) {
    description =
      people === null
        ? data.organizationName
        : t("adminOverview.subtitle", {
            organization: data.organizationName,
            total: new Intl.NumberFormat(locale).format(people.total),
          });
  }
  return (
    <div className="flex flex-col gap-5">
      <PageHead title={t("adminOverview.title")} description={description} crumb />
      {data === undefined ? <KpiSkeleton /> : <Kpis data={data} />}
      <div className="flex flex-wrap gap-3">
        {data === undefined || data.dailyActive !== null ? (
          <SettingsCard
            title={t("adminOverview.daily.title")}
            headerClassName="border-b-0 pb-2.5"
            className="min-w-0 flex-[2_1_32.5rem]"
          >
            {data?.dailyActive ? (
              <DailyActive daily={data.dailyActive} />
            ) : (
              <Waiting query={query} />
            )}
          </SettingsCard>
        ) : null}
        <SettingsCard
          title={t("adminOverview.attention.title")}
          headerClassName="border-b-0 pb-2.5"
          className="min-w-0 flex-[1_1_18.75rem] overflow-hidden"
        >
          {data === undefined ? (
            <Waiting query={query} />
          ) : (
            <AttentionList items={data.attention} />
          )}
        </SettingsCard>
      </div>
      <div className="flex flex-wrap gap-3">
        {data === undefined ? (
          <>
            <SettingsCard
              title={t("adminOverview.recent.title")}
              headerClassName="border-b-0 pb-2.5"
              className="min-w-0 flex-[2_1_32.5rem]"
            >
              <Waiting query={query} />
            </SettingsCard>
            <SettingsCard
              title={t("adminOverview.storage.title")}
              headerClassName="border-b-0 pb-2.5"
              className="min-w-0 flex-[1_1_18.75rem]"
            >
              <Waiting query={query} />
            </SettingsCard>
          </>
        ) : (
          <>
            {data.recentChanges === null ? null : (
              <RecentChanges entries={data.recentChanges} />
            )}
            {data.storage === null ? null : <StorageCard usage={data.storage} />}
          </>
        )}
      </div>
    </div>
  );
}
