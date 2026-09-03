import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
import { getTest, listVersions, previewTest } from "@/features/tests/api";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatDateTime } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";
import { PageHeader } from "@/components/shared/PageHeader";

/**
 * §8's test detail: what a student would receive, and the history of what they
 * have received before.
 *
 * The preview reads `/preview`, which renders the PUBLISHED version rather than
 * the draft — so a teacher checking a live test sees the live test, not the
 * edits they made this morning and have not published.
 */
export default function TestDetailPage() {
  const { t, i18n } = useTranslation();
  const { id = "" } = useParams();
  const locale = currentLocale(i18n.language);

  const test = useQuery({
    queryKey: ["admin-test", id],
    queryFn: ({ signal }) => getTest(id, signal),
  });
  const versions = useQuery({
    queryKey: ["admin-test-versions", id],
    queryFn: ({ signal }) => listVersions(id, signal),
  });
  const preview = useQuery({
    queryKey: ["admin-test-preview", id],
    queryFn: ({ signal }) => previewTest(id, undefined, signal),
    retry: false,
  });

  if (test.isPending) {
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }
  if (test.isError) {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-sm">
          {t("tests.detailFailed")}
        </p>
        <Button variant="outline" size="sm" onClick={() => void test.refetch()}>
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  const notPublished =
    preview.error instanceof ApiError && preview.error.code === "TEST_NOT_PUBLISHED";

  return (
    <>
      <PageHeader
        title={test.data.title}
        backTo="/admin/tests"
        meta={
          <Badge variant={test.data.status === "published" ? "success" : "secondary"}>
            {t(`builder.${test.data.status}`)}
          </Badge>
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
          </Button>
        }
      />

      <div className="grid gap-5 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-2">
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

          {preview.isPending ? (
            <p
              role="status"
              aria-live="polite"
              className="text-muted-foreground text-sm"
            >
              {t("common.loading")}
            </p>
          ) : notPublished ? (
            <div className="space-y-3">
              <p className="text-muted-foreground text-sm">{t("tests.notPublished")}</p>
              <Button asChild size="sm">
                <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
              </Button>
            </div>
          ) : preview.isError ? (
            <p role="alert" className="text-destructive text-sm">
              {t("tests.previewFailed")}
            </p>
          ) : (
            <StudentPreview questions={preview.data.questions} />
          )}
        </div>

        <Card asChild className="gap-0 py-0">
          <section aria-labelledby="versions-heading" className="self-start">
            <div className="px-5 pt-4 pb-3">
              <h2
                id="versions-heading"
                className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
              >
                {t("tests.versionHistory")}
              </h2>
            </div>
            <div className="px-5 pb-4">
              <VersionHistory query={versions} locale={locale} />
            </div>
          </section>
        </Card>
      </div>
    </>
  );
}

function VersionHistory({
  query,
  locale,
}: {
  query: UseQueryResult<Awaited<ReturnType<typeof listVersions>>>;
  locale: Locale;
}) {
  const { t } = useTranslation();
  if (query.isPending) {
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }
  if (query.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("tests.loadFailed")}
      </p>
    );
  }
  if (query.data.items.length === 0) {
    return <p className="text-muted-foreground text-sm">{t("tests.noVersions")}</p>;
  }
  return (
    <ol className="space-y-3">
      {query.data.items.map((version) => (
        <li key={version.id} className="text-sm">
          <div className="flex items-baseline justify-between gap-3">
            <span className="font-medium tabular-nums">
              {t("tests.versionNumber", { n: version.version })}
            </span>
            <span className="text-muted-foreground text-xs tabular-nums">
              {version.questionCount} · {version.totalPoints}
            </span>
          </div>
          <p className="text-muted-foreground text-xs">
            {formatDateTime(version.publishedAt, locale)} · {version.publishedBy}
          </p>
        </li>
      ))}
    </ol>
  );
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
