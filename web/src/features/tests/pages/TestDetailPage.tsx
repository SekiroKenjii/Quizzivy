import { useState } from "react";
import { History } from "lucide-react";
import { PageAside } from "@/components/shared/PageAside";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useParams, useSearchParams } from "react-router";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
import { getTest, listVersions, previewTest } from "@/features/tests/api";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDateTime } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";
import { ListSkeleton, LoadError, QueryStates } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { StatusBadge } from "@/components/shared/StatusBadge";

/**
 * §8's test detail: what a student would receive, and the history of what they
 * have received before.
 */
export default function TestDetailPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const locale = useLocale();
  const { hash } = useLocation();
  const [historyOpen, setHistoryOpen] = useState(hash === "#versions");
  const [params, setParams] = useSearchParams();
  const requestedVersion = Number(params.get("version"));
  const version =
    Number.isSafeInteger(requestedVersion) && requestedVersion > 0
      ? requestedVersion
      : undefined;

  const test = useQuery({
    queryKey: ["admin-test", id],
    queryFn: ({ signal }) => getTest(id, signal),
  });
  const versions = useQuery({
    queryKey: ["admin-test-versions", id],
    queryFn: ({ signal }) => listVersions(id, signal),
  });
  const preview = useQuery({
    queryKey: ["admin-test-preview", id, version],
    queryFn: ({ signal }) => previewTest(id, version, signal),
    retry: false,
  });

  if (test.isPending) {
    return <ListSkeleton rows={6} />;
  }
  if (test.isError) {
    return (
      <LoadError error={test.error} onRetry={() => void test.refetch()}>
        {t("tests.detailFailed")}
      </LoadError>
    );
  }

  const notPublished =
    preview.error instanceof ApiError && preview.error.code === "TEST_NOT_PUBLISHED";

  return (
    <>
      <PageHeader
        title={test.data.title}
        backTo="/admin/tests"
        meta={<StatusBadge kind="test" status={test.data.status} />}
        actions={
          <>
            <Button
              size="sm"
              variant="outline"
              className="lg:hidden"
              onClick={() => setHistoryOpen(true)}
            >
              <History aria-hidden="true" />
              {t("tests.versionHistory")}
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
            </Button>
          </>
        }
      />

      <div className="space-y-4">
        <div>
          <h2 className="text-[0.9375rem] font-semibold tracking-[-0.01em]">
            {t("tests.preview")}
          </h2>
          <p className="text-muted-foreground mt-0.5 text-sm">
            {preview.data
              ? t("tests.previewOf", { n: preview.data.version })
              : t("tests.previewNote")}
          </p>
        </div>

        {notPublished && !preview.isPending ? (
          <div className="space-y-3">
            <p className="text-muted-foreground text-sm">{t("tests.notPublished")}</p>
            <Button asChild size="sm">
              <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
            </Button>
          </div>
        ) : (
          <QueryStates
            query={preview}
            skeleton={<ListSkeleton rows={4} />}
            failed={t("tests.previewFailed")}
          >
            {(data) => <StudentPreview questions={data.questions} />}
          </QueryStates>
        )}
      </div>
      <PageAside
        label={t("tests.versionHistory")}
        hideBelow="lg"
        sheet={{ open: historyOpen, onOpenChange: setHistoryOpen }}
      >
        <h2 className="text-sm font-semibold">{t("tests.versionHistory")}</h2>
        <VersionHistory
          query={versions}
          locale={locale}
          selected={version ?? test.data.currentVersion}
          onSelect={(value) => {
            setParams((current) => {
              const next = new URLSearchParams(current);
              next.set("version", String(value));
              return next;
            });
            setHistoryOpen(false);
          }}
        />
      </PageAside>
    </>
  );
}

function VersionHistory({
  query,
  locale,
  selected,
  onSelect,
}: Readonly<{
  query: UseQueryResult<Awaited<ReturnType<typeof listVersions>>>;
  locale: Locale;
  selected: number;
  onSelect: (version: number) => void;
}>) {
  const { t } = useTranslation();
  if (query.isPending) {
    return <ListSkeleton rows={2} />;
  }
  if (query.isError) {
    return (
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("tests.loadFailed")}
      </LoadError>
    );
  }
  if (query.data.items.length === 0) {
    return <p className="text-muted-foreground text-sm">{t("tests.noVersions")}</p>;
  }
  return (
    <ol className="space-y-3">
      {query.data.items.map((version) => (
        <li key={version.id} className="text-sm">
          <button
            type="button"
            aria-pressed={selected === version.version}
            onClick={() => onSelect(version.version)}
            className="hover:bg-accent aria-pressed:bg-accent flex w-full items-baseline justify-between gap-3 rounded-md p-2 text-left"
          >
            <span className="font-medium tabular-nums">
              {t("tests.versionNumber", { n: version.version })}
            </span>
            <span className="text-muted-foreground text-xs tabular-nums">
              {version.questionCount} · {version.totalPoints}
            </span>
          </button>
          <p className="text-muted-foreground px-2 text-xs">
            {formatDateTime(version.publishedAt, locale)} · {version.publishedBy}
          </p>
        </li>
      ))}
    </ol>
  );
}
