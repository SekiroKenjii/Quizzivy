import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { PageHead } from "@/layouts/shell/PageHead";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { ApiError } from "@/lib/api/errors";
import { nfc } from "@/lib/nfc";
import {
  getWordImport,
  getWordImportReview,
  type ImportReview,
  type WordImport,
} from "../../api";
import { CommittedCard } from "../../components/confirm/CommittedCard";
import { ConfirmPreview } from "../../components/confirm/ConfirmPreview";
import { ConfirmSummary } from "../../components/confirm/ConfirmSummary";
import { useImportCommit } from "../../useImportCommit";

/**
 * ImportConfirmPage is an import's "Preview and create": the saved review's
 * draft as students will see it, its summary, and "Create draft test", or the
 * draft already created. It reads the import and its review afresh on entry,
 * so it never shows a review cached before the teacher's last save, and
 * follows them after, so a conflict's re-read shows the newer review.
 */
export default function ImportConfirmPage() {
  const { id = "" } = useParams();
  const { t } = useTranslation();
  const client = useQueryClient();
  const value = useQuery({
    queryKey: ["word-import", id],
    queryFn: ({ signal }) => getWordImport(id, signal),
    refetchOnMount: "always",
  });
  const review = useQuery({
    queryKey: ["word-import-review", id],
    queryFn: ({ signal }) => getWordImportReview(id, signal),
    refetchOnMount: "always",
  });
  const updates = () =>
    client.getQueryState(["word-import-review", id])?.dataUpdateCount ?? 0;
  const [cachedUpdates] = useState(updates);
  const fresh = updates() > cachedUpdates;
  useCrumbs(
    value.data
      ? [
          { label: nfc(value.data.title), to: `/teacher/imports/${id}/review` },
          { label: t("imports.confirm.crumb") },
        ]
      : null,
  );

  if (value.isPending || (review.isPending && !review.isError))
    return <ConfirmSkeleton />;
  if (value.isError) {
    if (value.error instanceof ApiError && value.error.status === 404)
      return (
        <EmptyState
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
            </Button>
          }
        >
          {t("imports.notFound")}
        </EmptyState>
      );
    return (
      <LoadError error={value.error} onRetry={() => void value.refetch()}>
        {t("imports.detailFailed")}
      </LoadError>
    );
  }
  if (!fresh && review.isFetching) return <ConfirmSkeleton />;
  const data = fresh ? review.data : undefined;
  if (data) return <ConfirmWorkspace value={value.data} review={data} />;
  if (value.data.status === "committed" && value.data.testId !== undefined)
    return (
      <ConfirmFrame id={id} review={null}>
        <CommittedCard
          testId={value.data.testId}
          title={value.data.title}
          summary={null}
        />
      </ConfirmFrame>
    );
  return (
    <ConfirmUnavailable
      id={id}
      removed={value.data.filesRemovedAt !== undefined}
      error={review.error}
      onRetry={() => void review.refetch()}
    />
  );
}

function ConfirmUnavailable({
  id,
  removed,
  error,
  onRetry,
}: Readonly<{
  id: string;
  removed: boolean;
  error: Error | null;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  if (removed || (error instanceof ApiError && error.code === "IMPORT_FILES_REMOVED"))
    return (
      <EmptyState
        action={
          <Button asChild size="sm" variant="outline">
            <Link to={`/teacher/imports/${id}`}>
              {t("imports.retention.viewImport")}
            </Link>
          </Button>
        }
      >
        {t("imports.retention.reviewRemoved")}
      </EmptyState>
    );
  if (error instanceof ApiError && error.code === "IMPORT_NOT_PROCESSED")
    return (
      <EmptyState
        hint={t("imports.review.notReadyHint")}
        action={
          <Button asChild size="sm" variant="outline">
            <Link to={`/teacher/imports/${id}`}>
              {t("imports.review.viewProgress")}
            </Link>
          </Button>
        }
      >
        {t("imports.review.notReady")}
      </EmptyState>
    );
  return (
    <LoadError error={error} onRetry={onRetry}>
      {t("imports.review.loadFailed")}
    </LoadError>
  );
}

function ConfirmWorkspace({
  value,
  review,
}: Readonly<{ value: WordImport; review: ImportReview }>) {
  const revision = useRef(review.revision);
  useEffect(() => {
    revision.current = review.revision;
  }, [review.revision]);
  const { state, commit } = useImportCommit(value.id, revision);
  const committed = value.status === "committed" ? value.testId : undefined;
  const testId = state.phase === "done" ? state.result.testId : committed;
  return (
    <ConfirmFrame id={value.id} review={review}>
      {testId !== undefined ? (
        <CommittedCard
          testId={testId}
          title={review.draft.title}
          summary={review.summary}
        />
      ) : (
        <ConfirmSummary
          importId={value.id}
          review={review}
          underReview={value.status === "needs_review"}
          state={state}
          onCommit={() => void commit()}
        />
      )}
    </ConfirmFrame>
  );
}

function ConfirmFrame({
  id,
  review,
  children,
}: Readonly<{ id: string; review: ImportReview | null; children: ReactNode }>) {
  const { t } = useTranslation();
  return (
    <div className="@container/confirm flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("imports.confirm.title")}
        description={t("imports.confirm.body")}
        actions={
          <Button asChild variant="outline" className="text-ui h-9">
            <Link to={`/teacher/imports/${id}/review`}>
              <ArrowLeft aria-hidden="true" />
              {t("imports.confirm.backToReview")}
            </Link>
          </Button>
        }
      />
      {review === null ? (
        <aside className="flex w-full max-w-90 min-w-0 flex-col gap-3">
          {children}
        </aside>
      ) : (
        <div className="grid min-w-0 items-start gap-4 @min-[1000px]/confirm:grid-cols-[minmax(0,1fr)_360px]">
          <ConfirmPreview sections={review.draft.sections} />
          <aside className="flex min-w-0 flex-col gap-3">{children}</aside>
        </div>
      )}
    </div>
  );
}

function ConfirmSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="@container/confirm flex flex-col gap-4"
    >
      <Skeleton className="h-12 w-2/3" />
      <div className="grid gap-4 @min-[1000px]/confirm:grid-cols-[minmax(0,1fr)_360px]">
        <Skeleton className="h-96 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </div>
    </div>
  );
}
