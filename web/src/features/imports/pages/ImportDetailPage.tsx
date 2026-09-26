import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Link, useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, failureMessage } from "@/lib/api/errors";
import {
  cancelWordImport,
  getWordImport,
  processWordImport,
  type ImportRetention,
  type WordImport,
} from "../api";
import {
  CAPABILITIES_POLL_MS,
  refreshAvailability,
  useImportAvailability,
  useImportRetention,
} from "../availability";
import { FilesRemovedNotice } from "../components/FilesRemovedNotice";
import { ImportStatusBadge } from "../components/ImportStatusBadge";
import { ProcessingOffNotice } from "../components/ProcessingOffNotice";
import { ProcessingPanel } from "../components/ProcessingPanel";
import { ReprocessNotice } from "../components/ReprocessNotice";
import { SourceIntake } from "../components/SourceIntake";
import { SourcesList } from "../components/SourcesList";
import { StaleNotice } from "../components/StaleNotice";
import { storeImport } from "../queries";
import {
  hasDraft,
  IMPORT_POLL_MS,
  isActiveStatus,
  isRetryable,
  reprocessOutcome,
  runErrorKey,
} from "../status";

type Store = (next: WordImport) => Promise<void>;

export default function ImportDetailPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const client = useQueryClient();
  const processing = useImportAvailability() !== "reviewOnly";
  const query = useQuery({
    queryKey: ["word-import", id],
    queryFn: ({ signal }) => getWordImport(id, signal),
    refetchInterval: (current) =>
      current.state.data !== undefined && isActiveStatus(current.state.data.status)
        ? IMPORT_POLL_MS
        : false,
    refetchIntervalInBackground: false,
  });
  const status = query.data?.status;

  useEffect(() => {
    if (status === "needs_review")
      client.removeQueries({ queryKey: ["word-import-review", id] });
  }, [client, id, status]);

  const store: Store = (next) => storeImport(client, next);

  if (query.data === undefined) {
    if (query.isPending)
      return (
        <div role="status" aria-label={t("common.loading")} className="space-y-3">
          <Skeleton className="h-8 w-72" />
          <Skeleton className="h-40 w-full" />
        </div>
      );
    if (query.error instanceof ApiError && query.error.status === 404)
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
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("imports.detailFailed")}
      </LoadError>
    );
  }
  const value = query.data;
  const outcome = reprocessOutcome(value);
  return (
    <>
      <PageHeader
        title={value.title}
        meta={<ImportStatusBadge status={value.status} />}
        backTo="/admin/imports"
        backLabel={t("imports.backToHistory")}
      />
      <div className="mx-auto max-w-3xl space-y-6">
        <p role="status" aria-live="polite" className="sr-only">
          {outcome === null
            ? t(`imports.status.${value.status}`)
            : t(`imports.reprocess.${outcome}`)}
        </p>
        {query.isError ? <StaleNotice onRetry={() => void query.refetch()} /> : null}
        <StatePanel value={value} onChange={store} processing={processing} />
        {value.filesRemovedAt === undefined ? null : (
          <FilesRemovedNotice at={value.filesRemovedAt} />
        )}
        {value.status === "awaiting_sources" && processing ? null : (
          <section aria-labelledby="import-sources" className="space-y-2">
            <h2 id="import-sources" className="text-sm font-medium">
              {t("imports.sources.title")}
            </h2>
            <SourcesList
              importId={value.id}
              sources={value.sources}
              pendingUploads={value.pendingUploads}
              removed={value.filesRemovedAt !== undefined}
            />
          </section>
        )}
      </div>
    </>
  );
}

function Panel({
  title,
  description,
  children,
}: Readonly<{
  title: string;
  description?: string | undefined;
  children?: ReactNode;
}>) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description === undefined ? null : (
          <CardDescription>{description}</CardDescription>
        )}
      </CardHeader>
      {children === undefined ? null : (
        <CardContent className="flex flex-col gap-4 pt-1">{children}</CardContent>
      )}
    </Card>
  );
}

function StatePanel({
  value,
  onChange,
  processing,
}: Readonly<{ value: WordImport; onChange: Store; processing: boolean }>) {
  const { t } = useTranslation();
  switch (value.status) {
    case "awaiting_sources":
    case "failed":
      return <IntakePanel value={value} onChange={onChange} processing={processing} />;
    case "queued":
    case "processing":
      return <ProcessingState value={value} onChange={onChange} />;
    case "committing":
      return (
        <Panel
          title={t("imports.detail.committingTitle")}
          description={t("imports.detail.committingBody")}
        />
      );
    case "needs_review":
      return <ReadyPanel value={value} onChange={onChange} processing={processing} />;
    case "committed":
      return (
        <Panel
          title={t("imports.detail.committedTitle")}
          description={t("imports.detail.committedBody")}
        >
          {value.testId === undefined ? null : (
            <Button asChild className="self-start">
              <Link to={`/admin/tests/${value.testId}/edit`}>
                {t("imports.detail.openBuilder")}
              </Link>
            </Button>
          )}
        </Panel>
      );
    case "cancelled":
      return <ClosedPanel value={value} processing={processing} />;
  }
}

function IdleWarning() {
  const { t } = useTranslation();
  const retention = useImportRetention();
  if (retention === undefined) return null;
  return (
    <p className="text-muted-foreground text-xs">
      {t("imports.retention.idleWarning", { days: retention.idleDays })}
    </p>
  );
}

function ReadyPanel({
  value,
  onChange,
  processing,
}: Readonly<{ value: WordImport; onChange: Store; processing: boolean }>) {
  const { t } = useTranslation();
  const reprocessFailed = reprocessOutcome(value) === "failed";
  return (
    <Panel
      title={t("imports.detail.readyTitle")}
      description={t("imports.detail.readyBody")}
    >
      <ReprocessNotice value={value} className="bg-muted/40 rounded-md p-3 text-sm" />
      {reprocessFailed ? (
        <RetryRun
          value={value}
          onChange={onChange}
          label={t("imports.detail.retryReprocess")}
          canRetry={processing}
        />
      ) : null}
      {reprocessFailed && !processing ? (
        <ProcessingOffNotice>
          {t("imports.availability.reprocessOff")}
        </ProcessingOffNotice>
      ) : null}
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild>
          <Link to={`/admin/imports/${value.id}/review`}>
            {t("imports.detail.continueReview")}
          </Link>
        </Button>
        <CloseImport value={value} onChange={onChange} />
      </div>
      <IdleWarning />
    </Panel>
  );
}

function closedBody(
  t: TFunction,
  value: WordImport,
  retention: ImportRetention | undefined,
): string | undefined {
  if (value.filesRemovedAt !== undefined) return undefined;
  if (value.closedIdle === true)
    return retention === undefined
      ? t("imports.detail.closedIdleBodyPlain")
      : t("imports.detail.closedIdleBody", { days: retention.idleDays });
  const days = retention?.afterCancelDays;
  if (hasDraft(value))
    return days === undefined
      ? t("imports.detail.cancelledBodyReview")
      : t("imports.detail.cancelledBodyReviewDays", { days });
  return days === undefined
    ? t("imports.detail.cancelledBody")
    : t("imports.detail.cancelledBodyDays", { days });
}

function closeBody(
  t: TFunction,
  value: WordImport,
  retention: ImportRetention | undefined,
): string {
  const days = retention?.afterCancelDays;
  if (hasDraft(value))
    return days === undefined
      ? t("imports.detail.closeBodyReview")
      : t("imports.detail.closeBodyReviewDays", { days });
  return days === undefined
    ? t("imports.detail.closeBody")
    : t("imports.detail.closeBodyDays", { days });
}

function cancelBody(t: TFunction, retention: ImportRetention | undefined): string {
  return retention === undefined
    ? t("imports.detail.cancelBody")
    : t("imports.detail.cancelBodyDays", { days: retention.afterCancelDays });
}

function ClosedPanel({
  value,
  processing,
}: Readonly<{ value: WordImport; processing: boolean }>) {
  const { t } = useTranslation();
  const retention = useImportRetention();
  const removed = value.filesRemovedAt !== undefined;
  const reviewLink =
    hasDraft(value) && !removed ? (
      <Button asChild variant="outline">
        <Link to={`/admin/imports/${value.id}/review`}>
          {t("imports.detail.viewReview")}
        </Link>
      </Button>
    ) : null;
  return (
    <Panel
      title={t("imports.detail.cancelledTitle")}
      description={closedBody(t, value, retention)}
    >
      {processing ? null : (
        <ProcessingOffNotice>
          {t("imports.availability.startOverOff")}
        </ProcessingOffNotice>
      )}
      <div className="flex flex-wrap items-center gap-3 empty:hidden">
        {processing ? (
          <Button asChild variant="outline">
            <Link to="/admin/imports/new">{t("imports.detail.startOver")}</Link>
          </Button>
        ) : null}
        {reviewLink}
      </div>
    </Panel>
  );
}

function useCancel(value: WordImport, onChange: Store, fallback: string) {
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: () => cancelWordImport(value.id, { expectedRevision: value.revision }),
    onSuccess: async (next) => {
      setOpen(false);
      await onChange(next);
    },
    onError: (cause) => {
      setOpen(false);
      setError(failureMessage(cause, fallback));
      void client.invalidateQueries({ queryKey: ["word-import", value.id] });
    },
  });
  const ask = () => {
    setError(null);
    setOpen(true);
  };
  return { open, setOpen, error, mutation, ask };
}

function CloseImport({
  value,
  onChange,
}: Readonly<{ value: WordImport; onChange: Store }>) {
  const { t } = useTranslation();
  const retention = useImportRetention();
  const close = useCancel(value, onChange, t("imports.detail.closeFailed"));
  return (
    <>
      <Button
        variant="ghost"
        className="text-muted-foreground self-start"
        disabled={close.mutation.isPending}
        onClick={close.ask}
      >
        {t("imports.detail.close")}
      </Button>
      {close.error === null ? null : (
        <p role="alert" className="text-sm">
          {close.error}
        </p>
      )}
      <ConfirmDialog
        open={close.open}
        onOpenChange={close.setOpen}
        title={t("imports.detail.closeTitle")}
        description={closeBody(t, value, retention)}
        confirmLabel={t("imports.detail.closeConfirm")}
        cancelLabel={t("imports.detail.closeKeep")}
        destructive
        pending={close.mutation.isPending}
        onConfirm={() => close.mutation.mutate()}
      />
    </>
  );
}

function ProcessingState({
  value,
  onChange,
}: Readonly<{ value: WordImport; onChange: Store }>) {
  const { t } = useTranslation();
  const retention = useImportRetention();
  const waitingForWorker =
    useImportAvailability(CAPABILITIES_POLL_MS) === "reviewOnly" &&
    value.status === "queued";
  const reprocess = hasDraft(value);
  const cancel = useCancel(value, onChange, t("imports.detail.cancelFailed"));
  const body = reprocess
    ? t("imports.detail.reprocessBody")
    : t("imports.detail.processingBody");
  return (
    <Panel
      title={
        reprocess
          ? t("imports.detail.reprocessTitle")
          : t("imports.detail.processingTitle")
      }
      description={waitingForWorker ? undefined : body}
    >
      <ProcessingPanel run={value.run} />
      {waitingForWorker ? (
        <ProcessingOffNotice>{t("imports.availability.queuedOff")}</ProcessingOffNotice>
      ) : null}
      {cancel.error === null ? null : (
        <p role="alert" className="text-sm">
          {cancel.error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link to="/admin/imports">{t("imports.detail.leave")}</Link>
        </Button>
        {reprocess ? (
          <Button asChild variant="outline" size="sm">
            <Link to={`/admin/imports/${value.id}/review`}>
              {t("imports.detail.viewReview")}
            </Link>
          </Button>
        ) : null}
        <Button
          variant="ghost"
          size="sm"
          disabled={cancel.mutation.isPending}
          onClick={cancel.ask}
        >
          {reprocess ? t("imports.detail.stopReprocess") : t("imports.detail.cancel")}
        </Button>
      </div>
      <ConfirmDialog
        open={cancel.open}
        onOpenChange={cancel.setOpen}
        title={
          reprocess
            ? t("imports.detail.stopReprocessTitle")
            : t("imports.detail.cancelTitle")
        }
        description={
          reprocess ? t("imports.detail.stopReprocessBody") : cancelBody(t, retention)
        }
        confirmLabel={
          reprocess
            ? t("imports.detail.stopReprocessConfirm")
            : t("imports.detail.cancelConfirm")
        }
        cancelLabel={t("imports.detail.cancelKeep")}
        destructive={!reprocess}
        pending={cancel.mutation.isPending}
        onConfirm={() => cancel.mutation.mutate()}
      />
    </Panel>
  );
}

function useRetry(value: WordImport, onChange: Store) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const attempt = useRef<{ requestId: string; revision: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const keyPaper = value.run?.keyPaper;
  const mutation = useMutation({
    mutationFn: () => {
      if (attempt.current?.revision !== value.revision)
        attempt.current = { requestId: crypto.randomUUID(), revision: value.revision };
      return processWordImport(value.id, {
        requestId: attempt.current.requestId,
        expectedRevision: value.revision,
        ...(keyPaper === undefined ? {} : { keyPaper }),
      });
    },
    onSuccess: async (next) => {
      setError(null);
      await onChange(next);
    },
    onError: (cause) => {
      setError(failureMessage(cause, t("imports.detail.retryFailed")));
      refreshAvailability(client, cause);
      if (cause instanceof ApiError && cause.status === 409)
        void client.invalidateQueries({ queryKey: ["word-import", value.id] });
    },
  });
  return { mutation, error };
}

function RetryRun({
  value,
  onChange,
  label,
  canRetry,
}: Readonly<{
  value: WordImport;
  onChange: Store;
  label?: string;
  canRetry: boolean;
}>) {
  const { t } = useTranslation();
  const retry = useRetry(value, onChange);
  const errorCode = value.run?.errorCode;
  return (
    <>
      {errorCode === undefined ? null : (
        <p className="text-muted-foreground text-xs">
          {t("imports.detail.errorCode", { code: errorCode })}
        </p>
      )}
      {retry.error === null ? null : (
        <p role="alert" className="text-sm">
          {retry.error}
        </p>
      )}
      {canRetry && isRetryable(errorCode) ? (
        <Button
          className="self-start"
          variant={label === undefined ? "default" : "outline"}
          disabled={retry.mutation.isPending}
          onClick={() => retry.mutation.mutate()}
        >
          {retry.mutation.isPending
            ? t("imports.detail.retrying")
            : (label ?? t("imports.detail.retry"))}
        </Button>
      ) : null}
    </>
  );
}

function IntakePanel({
  value,
  onChange,
  processing,
}: Readonly<{ value: WordImport; onChange: Store; processing: boolean }>) {
  const { t } = useTranslation();
  const headingId = useId();
  const failed = value.status === "failed";
  const awaitingBody = processing ? t("imports.detail.awaitingBody") : undefined;
  return (
    <Panel
      title={
        failed ? t("imports.detail.failedTitle") : t("imports.detail.awaitingTitle")
      }
      description={
        failed
          ? t(`imports.runError.${runErrorKey(value.run?.errorCode)}`)
          : awaitingBody
      }
    >
      {processing ? null : (
        <ProcessingOffNotice>{t("imports.availability.intakeOff")}</ProcessingOffNotice>
      )}
      {failed ? (
        <RetryRun value={value} onChange={onChange} canRetry={processing} />
      ) : null}
      {processing ? (
        <section
          aria-labelledby={failed ? headingId : undefined}
          className={failed ? "space-y-3 border-t pt-4" : undefined}
        >
          {failed ? (
            <div className="space-y-1">
              <h2 id={headingId} className="text-sm font-medium">
                {t("imports.detail.replaceTitle")}
              </h2>
              <p className="text-muted-foreground text-xs leading-relaxed">
                {t("imports.detail.replaceHint")}
              </p>
            </div>
          ) : null}
          <SourceIntake
            key={value.id}
            existing={value}
            replacing={failed}
            onChanged={(next) => void onChange(next)}
            onStarted={(next) => void onChange(next)}
          />
        </section>
      ) : null}
      <CloseImport value={value} onChange={onChange} />
      <IdleWarning />
    </Panel>
  );
}
