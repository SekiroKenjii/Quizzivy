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
import { Callout } from "@/components/shared/Callout";
import { Badge } from "@/components/ui/badge";
import { ClipboardPaste, FileText, ArrowRight } from "lucide-react";
import { formatRelative, formatDateTime } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, failureMessage } from "@/lib/api/errors";
import {
  cancelWordImport,
  getWordImport,
  getWordImportReview,
  type ImportReview,
  type ImportReviewSummary,
  processWordImport,
  type ImportRetention,
  type WordImport,
} from "../../api";
import {
  CAPABILITIES_POLL_MS,
  refreshAvailability,
  useImportAvailability,
  useImportRetention,
} from "../../availability";
import { FilesRemovedNotice } from "../../components/FilesRemovedNotice";
import { ProcessingOffNotice } from "../../components/ProcessingOffNotice";
import { ProcessingPanel } from "../../components/ProcessingPanel";
import { ReprocessNotice } from "../../components/ReprocessNotice";
import { SourceIntake } from "../../components/SourceIntake";
import { SourcesList } from "../../components/SourcesList";
import { StaleNotice } from "../../components/StaleNotice";
import { storeImport } from "../../queries";
import {
  hasDraft,
  IMPORT_POLL_MS,
  isActiveStatus,
  isRetryable,
  reprocessOutcome,
  runErrorKey,
  PROCESSING_STAGES,
} from "../../status";

type Store = (next: WordImport) => Promise<void>;

export default function ImportDetailPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const client = useQueryClient();
  const processing = useImportAvailability() === "on";
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
        <div
          role="status"
          aria-label={t("common.loading")}
          data-scale="deck"
          className="mx-auto flex w-full max-w-[720px] flex-col gap-3 focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!"
        >
          <Skeleton className="h-8 w-72" />
          <Skeleton className="h-40 w-full" />
        </div>
      );
    if (query.error instanceof ApiError && query.error.status === 404)
      return (
        <div className="mx-auto w-full max-w-[720px] min-w-0" data-scale="deck">
          <EmptyState
            action={
              <Button asChild size="sm" variant="outline">
                <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
              </Button>
            }
          >
            {t("imports.notFound")}
          </EmptyState>
        </div>
      );
    return (
      <div className="mx-auto w-full max-w-[720px] min-w-0" data-scale="deck">
        <LoadError error={query.error} onRetry={() => void query.refetch()}>
          {t("imports.detailFailed")}
        </LoadError>
      </div>
    );
  }
  const value = query.data;
  const outcome = reprocessOutcome(value);
  return (
    <div
      className="mx-auto flex w-full max-w-[720px] min-w-0 flex-col gap-4"
      data-scale="deck"
    >
      <DetailHead value={value} />
      <div className="flex min-w-0 flex-col gap-4">
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
    </div>
  );
}

function DetailHead({ value }: Readonly<{ value: WordImport }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const exam = value.sources.find((source) => source.role === "exam");
  const key = value.sources.find((source) => source.role === "answer_key");
  const started = value.run?.createdAt ?? value.createdAt;
  const text = exam?.format === "text";
  const name = text ? t("imports.detail.pastedText") : (exam?.filename ?? value.title);
  return (
    <header className="flex min-w-0 flex-col gap-3">
      <Link
        to="/teacher/imports"
        className="text-muted-fg hover:text-fg self-start text-sm"
      >
        {t("imports.backToHistory")}
      </Link>
      <div className="flex min-w-0 items-center gap-3">
        <span className="bg-info-soft text-info-ink grid size-10 shrink-0 place-items-center rounded-[10px]">
          {text ? (
            <ClipboardPaste className="size-[19px]" aria-hidden="true" />
          ) : (
            <FileText className="size-[19px]" aria-hidden="true" />
          )}
        </span>
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-semibold tracking-tight">
            <span className="block truncate" title={name}>
              {name}
            </span>
          </h1>
          <p
            className="text-muted-fg text-sm break-words"
            title={formatDateTime(started, locale)}
          >
            {t(
              value.run === undefined
                ? "imports.detail.created"
                : "imports.detail.started",
              { when: formatRelative(started, locale) },
            )}
            {text ? <> · {t("imports.detail.fromPastedText")}</> : null}
            {key ? (
              <> · {t("imports.history.withKey", { name: key.filename })}</>
            ) : null}
          </p>
          {name === value.title ? null : (
            <p className="text-muted-fg text-xs break-words">{value.title}</p>
          )}
        </div>
      </div>
    </header>
  );
}

function Panel({
  title,
  description,
  status,
  children,
}: Readonly<{
  title: string;
  description?: string | undefined;
  status?: ReactNode;
  children?: ReactNode;
}>) {
  return (
    <Card className="gap-3.5 py-5">
      <CardHeader className="px-5">
        {status}
        <CardTitle className="text-[17px] leading-snug">{title}</CardTitle>
        {description === undefined ? null : (
          <CardDescription>{description}</CardDescription>
        )}
      </CardHeader>
      {children === undefined ? null : (
        <CardContent className="flex flex-col gap-3.5 px-5 pt-0">
          {children}
        </CardContent>
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
              <Link to={`/teacher/tests/${value.testId}/edit`}>
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
  const processingOff = useProcessingOff();
  const review = useQuery({
    queryKey: [
      "word-import-ready-summary",
      value.id,
      value.draftRevision ?? 0,
      value.revision,
    ],
    queryFn: ({ signal }) => getWordImportReview(value.id, signal),
  });
  return (
    <Panel
      title={
        review.data === undefined
          ? t("imports.detail.readyTitle")
          : readyTitle(review.data.summary, t)
      }
      description={t("imports.detail.readyBody")}
      status={
        <Badge variant="success" className="w-fit self-start rounded-full!">
          {t("imports.status.needs_review")}
        </Badge>
      }
    >
      {review.data === undefined ? (
        <ReadySummaryState
          pending={review.isPending}
          onRetry={() => void review.refetch()}
        />
      ) : (
        <ReadyPills review={review.data} />
      )}
      {review.data !== undefined && review.isError ? (
        <StaleNotice onRetry={() => void review.refetch()} />
      ) : null}
      <ReprocessNotice value={value} className="bg-muted/40 rounded-md p-3 text-sm" />
      {reprocessFailed ? (
        <RetryRun
          value={value}
          onChange={onChange}
          label={t("imports.detail.retryReprocess")}
          canRetry={processing}
        />
      ) : null}
      {reprocessFailed && processingOff ? (
        <ProcessingOffNotice>
          {t("imports.availability.reprocessOff")}
        </ProcessingOffNotice>
      ) : null}
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild>
          <Link to={`/teacher/imports/${value.id}/review`}>
            {t("imports.detail.startReview")}
            <ArrowRight aria-hidden="true" />
          </Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/teacher/imports">{t("imports.detail.later")}</Link>
        </Button>
        <CloseImport value={value} onChange={onChange} />
      </div>
      <IdleWarning />
    </Panel>
  );
}

function ReadySummaryState({
  pending,
  onRetry,
}: Readonly<{ pending: boolean; onRetry: () => void }>) {
  const { t } = useTranslation();
  if (pending)
    return (
      <p role="status" className="text-muted-fg text-sm">
        {t("imports.detail.summaryLoading")}
      </p>
    );
  return (
    <div role="alert" className="flex flex-wrap items-center gap-2 text-sm">
      <span>{t("imports.detail.summaryFailed")}</span>
      <Button type="button" variant="outline" size="xs" onClick={onRetry}>
        {t("common.retry")}
      </Button>
    </div>
  );
}

function ReadyPills({ review }: Readonly<{ review: ImportReview }>) {
  const { t } = useTranslation();
  const notes = review.findings.filter(
    (finding) => finding.severity === "informational",
  ).length;
  const summary = review.summary;
  const complete =
    summary.blocking === 0 &&
    summary.answersMissing === 0 &&
    summary.answersConflicting === 0;
  return (
    <div className="flex flex-wrap gap-2">
      {complete ? (
        <Badge variant="success" className="rounded-full!">
          {t("imports.detail.everyAnswer")}
        </Badge>
      ) : null}
      {summary.blocking > 0 ? (
        <Badge variant="danger" className="rounded-full!">
          {t("imports.detail.needAction", { count: summary.blocking })}
        </Badge>
      ) : null}
      {summary.needsDecision > 0 ? (
        <Badge variant="warning" className="rounded-full!">
          {t("imports.detail.toConfirm", { count: summary.needsDecision })}
        </Badge>
      ) : null}
      {notes > 0 ? (
        <Badge variant="secondary" className="rounded-full!">
          {t("imports.detail.notes", { count: notes })}
        </Badge>
      ) : null}
    </div>
  );
}

function closedBody(
  t: TFunction,
  value: WordImport,
  retention: ImportRetention | undefined,
): string | undefined {
  if (value.filesRemovedAt !== undefined) return undefined;
  const prefix = isTextImport(value) ? "imports.detail.text" : "imports.detail";
  if (value.closedIdle === true)
    return retention === undefined
      ? t(`${prefix}.closedIdleBodyPlain`)
      : t(`${prefix}.closedIdleBody`, { days: retention.idleDays });
  const days = retention?.afterCancelDays;
  if (hasDraft(value))
    return days === undefined
      ? t(`${prefix}.cancelledBodyReview`)
      : t(`${prefix}.cancelledBodyReviewDays`, { days });
  return days === undefined
    ? t(`${prefix}.cancelledBody`)
    : t(`${prefix}.cancelledBodyDays`, { days });
}

function closeBody(
  t: TFunction,
  value: WordImport,
  retention: ImportRetention | undefined,
): string {
  const prefix = isTextImport(value) ? "imports.detail.text" : "imports.detail";
  const days = retention?.afterCancelDays;
  if (hasDraft(value))
    return days === undefined
      ? t(`${prefix}.closeBodyReview`)
      : t(`${prefix}.closeBodyReviewDays`, { days });
  return days === undefined
    ? t(`${prefix}.closeBody`)
    : t(`${prefix}.closeBodyDays`, { days });
}

function cancelBody(
  t: TFunction,
  value: WordImport,
  retention: ImportRetention | undefined,
): string {
  const prefix = isTextImport(value) ? "imports.detail.text" : "imports.detail";
  return retention === undefined
    ? t(`${prefix}.cancelBody`)
    : t(`${prefix}.cancelBodyDays`, { days: retention.afterCancelDays });
}

function ClosedPanel({
  value,
  processing,
}: Readonly<{ value: WordImport; processing: boolean }>) {
  const { t } = useTranslation();
  const retention = useImportRetention();
  const processingOff = useProcessingOff();
  const removed = value.filesRemovedAt !== undefined;
  const reviewLink =
    hasDraft(value) && !removed ? (
      <Button asChild variant="outline">
        <Link to={`/teacher/imports/${value.id}/review`}>
          {t("imports.detail.viewReview")}
        </Link>
      </Button>
    ) : null;
  return (
    <Panel
      status={
        <Badge variant="secondary" className="w-fit self-start rounded-full!">
          {t("imports.status.cancelled")}
        </Badge>
      }
      title={t("imports.detail.cancelledTitle")}
      description={closedBody(t, value, retention)}
    >
      {processingOff ? (
        <ProcessingOffNotice>
          {t("imports.availability.startOverOff")}
        </ProcessingOffNotice>
      ) : null}
      <div className="flex flex-wrap items-center gap-3">
        <Button asChild variant="outline">
          <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
        </Button>
        {processing ? (
          <Button asChild variant="outline">
            <Link
              to={
                isTextImport(value)
                  ? "/teacher/imports/new?source=paste"
                  : "/teacher/imports/new"
              }
            >
              {t("imports.detail.startOver")}
            </Link>
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
    <div className="flex flex-col gap-4">
      <h2 className="sr-only">
        {reprocess
          ? t("imports.detail.reprocessTitle")
          : t("imports.detail.processingTitle")}
      </h2>
      <ProcessingPanel
        run={value.run}
        exam={value.sources.find((source) => source.role === "exam")}
      />
      <Callout>{waitingForWorker ? t("imports.detail.waitingLeave") : body}</Callout>
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
          <Link to="/teacher/imports">{t("imports.detail.leave")}</Link>
        </Button>
        {reprocess ? (
          <Button asChild variant="outline" size="sm">
            <Link to={`/teacher/imports/${value.id}/review`}>
              {t("imports.detail.viewReview")}
            </Link>
          </Button>
        ) : null}
        <Button
          variant="ghost"
          className="text-danger-ink hover:bg-danger-soft"
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
          reprocess
            ? t("imports.detail.stopReprocessBody")
            : cancelBody(t, value, retention)
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
    </div>
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
        <p className="text-muted-foreground font-mono text-xs">
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
  const failed = value.status === "failed";
  const text = isTextImport(value);
  const processingOff = useProcessingOff();
  const awaitingKey = text
    ? "imports.detail.text.awaitingBody"
    : "imports.detail.awaitingBody";
  const awaitingBody = processing ? t(awaitingKey) : undefined;
  return (
    <Panel
      status={failed ? <FailureStage value={value} /> : undefined}
      title={failed ? failureTitle(value, t) : t("imports.detail.awaitingTitle")}
      description={failed ? failureDescription(value, t) : awaitingBody}
    >
      {processingOff ? (
        <ProcessingOffNotice>{t("imports.availability.intakeOff")}</ProcessingOffNotice>
      ) : null}
      {failed ? (
        <RetryRun value={value} onChange={onChange} canRetry={processing} />
      ) : null}
      {processing ? (
        <IntakeActions value={value} onChange={onChange} failed={failed} />
      ) : null}
      <CloseImport value={value} onChange={onChange} />
      <IdleWarning />
    </Panel>
  );
}

function IntakeActions({
  value,
  onChange,
  failed,
}: Readonly<{ value: WordImport; onChange: Store; failed: boolean }>) {
  const { t } = useTranslation();
  if (isTextImport(value))
    return failed ? (
      <Button asChild className="self-start">
        <Link to="/teacher/imports/new?source=paste">
          {t("imports.detail.startOver")}
        </Link>
      </Button>
    ) : (
      <RetryRun
        value={value}
        onChange={onChange}
        canRetry
        label={t("imports.upload.start")}
      />
    );
  return <FileIntake value={value} onChange={onChange} failed={failed} />;
}

function FileIntake({
  value,
  onChange,
  failed,
}: Readonly<{ value: WordImport; onChange: Store; failed: boolean }>) {
  const { t } = useTranslation();
  const headingId = useId();
  return (
    <>
      {failed ? (
        <Button asChild className="self-start">
          <a href={`#${headingId}`}>{t("imports.detail.replaceExam")}</a>
        </Button>
      ) : null}
      <section
        id={headingId}
        tabIndex={-1}
        aria-labelledby={failed ? `${headingId}-title` : undefined}
        className={failed ? "space-y-3 border-t pt-4" : undefined}
      >
        {failed ? (
          <div className="space-y-1">
            <h2 id={`${headingId}-title`} className="text-sm font-medium">
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
    </>
  );
}

function FailureStage({ value }: Readonly<{ value: WordImport }>) {
  const { t } = useTranslation();
  const stage = value.run?.stage;
  const labels = PROCESSING_STAGES.filter((candidate) =>
    candidate.runStages.some((runStage) => runStage === stage),
  ).map((candidate) =>
    t(
      isTextImport(value) && candidate.key === "validate"
        ? "imports.processing.textCheck"
        : `imports.processing.stage.${candidate.key}`,
    ),
  );
  const label =
    labels.length === 0 ? t("imports.processing.waitingToStart") : labels.join(" · ");
  return (
    <Badge
      variant="danger"
      className="w-fit self-start rounded-full! whitespace-normal"
    >
      {t("imports.detail.stoppedAt", { stage: label })}
    </Badge>
  );
}

function useProcessingOff(): boolean {
  const availability = useImportAvailability();
  return availability === "reviewOnly" || availability === "off";
}

function readyTitle(summary: ImportReviewSummary, t: TFunction): string {
  return t("imports.detail.found", {
    questions: t("imports.detail.questionCount", { count: summary.questions }),
    answers: t("imports.detail.answerCount", { count: summary.answersKnown }),
    sections: t("imports.detail.sectionCount", { count: summary.sections }),
  });
}

function isTextImport(value: WordImport): boolean {
  return value.sources.some(
    (source) => source.role === "exam" && source.format === "text",
  );
}

function failureTitle(value: WordImport, t: TFunction): string {
  const key = runErrorKey(value.run?.errorCode);
  return t(
    isTextImport(value)
      ? [`imports.textFailureTitle.${key}`, `imports.failureTitle.${key}`]
      : `imports.failureTitle.${key}`,
  );
}

function failureDescription(value: WordImport, t: TFunction): string {
  const key = runErrorKey(value.run?.errorCode);
  return t(
    isTextImport(value)
      ? [`imports.textRunError.${key}`, `imports.runError.${key}`]
      : `imports.runError.${key}`,
  );
}
