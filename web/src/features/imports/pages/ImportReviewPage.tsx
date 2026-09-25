import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { ApiError } from "@/lib/api/errors";
import { getWordImport, getWordImportReview } from "../api";
import { ReviewWorkspace } from "../components/review/ReviewWorkspace";
import { IMPORT_POLL_MS, isActiveStatus } from "../status";

export default function ImportReviewPage() {
  const { id = "" } = useParams();
  const client = useQueryClient();
  const [reloads, setReloads] = useState(0);
  return (
    <ReviewRoute
      key={`${id}:${reloads}`}
      id={id}
      onReload={() => {
        void client.resetQueries({ queryKey: ["word-import-review", id] });
        void client.invalidateQueries({ queryKey: ["word-import", id] });
        setReloads((current) => current + 1);
      }}
    />
  );
}

function ReviewRoute({ id, onReload }: Readonly<{ id: string; onReload: () => void }>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [generation, setGeneration] = useState(0);
  const value = useQuery({
    queryKey: ["word-import", id],
    queryFn: ({ signal }) => getWordImport(id, signal),
    refetchOnMount: "always",
    refetchInterval: (current) =>
      current.state.data !== undefined && isActiveStatus(current.state.data.status)
        ? IMPORT_POLL_MS
        : false,
    refetchIntervalInBackground: false,
  });
  const review = useQuery({
    queryKey: ["word-import-review", id],
    queryFn: ({ signal }) => getWordImportReview(id, signal),
    refetchOnMount: "always",
  });
  const updates = () =>
    client.getQueryState(["word-import-review", id])?.dataUpdateCount ?? 0;
  const [cachedUpdates] = useState(updates);
  const fresh = review.data !== undefined && updates() > cachedUpdates;

  if (value.data !== undefined && review.data !== undefined && fresh)
    return (
      <ReviewWorkspace
        key={`${id}:${generation}`}
        value={value.data}
        initial={review.data}
        onReload={onReload}
        onAdopted={(adopted) => {
          client.setQueryData(["word-import-review", id], adopted);
          setGeneration((current) => current + 1);
        }}
      />
    );
  if (value.isPending || review.isPending || (review.isFetching && !fresh))
    return (
      <div role="status" aria-label={t("common.loading")} className="space-y-3">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  if (value.isError) {
    if (value.error instanceof ApiError && value.error.status === 404)
      return (
        <EmptyState
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/admin/imports">{t("imports.backToHistory")}</Link>
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
  if (review.error instanceof ApiError && review.error.code === "IMPORT_NOT_PROCESSED")
    return (
      <EmptyState
        hint={t("imports.review.notReadyHint")}
        action={
          <Button asChild size="sm" variant="outline">
            <Link to={`/admin/imports/${id}`}>{t("imports.review.viewProgress")}</Link>
          </Button>
        }
      >
        {t("imports.review.notReady")}
      </EmptyState>
    );
  return (
    <LoadError error={review.error} onRetry={() => void review.refetch()}>
      {t("imports.review.loadFailed")}
    </LoadError>
  );
}
