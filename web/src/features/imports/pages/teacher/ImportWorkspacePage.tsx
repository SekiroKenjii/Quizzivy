import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Link, Navigate, useNavigate, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  CircleAlert,
  CircleX,
  CircleCheck,
  CircleHelp,
  ClipboardPaste,
  FileUp,
  Info,
  ShieldCheck,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout } from "@/components/shared/Callout";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { formatBytes } from "@/features/media/format";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { formatDateTime, formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import {
  cancelWordImport,
  getWordImport,
  getWordImportReview,
  processWordImport,
  type ImportRetention,
  type ImportReview,
  type ImportReviewSummary,
  type ImportSource,
  type WordImport,
} from "../../api";
import {
  CAPABILITIES_POLL_MS,
  refreshAvailability,
  useImportAvailability,
  useImportRetention,
} from "../../availability";
import { FilesRemovedNotice } from "../../components/FilesRemovedNotice";
import { ImportStatusBadge } from "../../components/ImportStatusBadge";
import { ProcessingOffNotice } from "../../components/ProcessingOffNotice";
import { ProcessingPanel } from "../../components/ProcessingPanel";
import { ReprocessNotice } from "../../components/ReprocessNotice";
import { SourceIntake, type IntakeParts } from "../../components/SourceIntake";
import { SourcesList, type SourcePill } from "../../components/SourcesList";
import { StaleNotice } from "../../components/StaleNotice";
import { Pane, PaneBody, WorkspaceGrid } from "../../components/Workspace";
import { listFormats } from "../../formats";
import { importName } from "../../importName";
import { importLimitsQuery, storeImport } from "../../queries";
import {
  hasDraft,
  IMPORT_POLL_MS,
  isActiveStatus,
  isRetryable,
  PROCESSING_STAGES,
  reprocessOutcome,
  runErrorKey,
} from "../../status";

type Store = (next: WordImport) => Promise<void>;

const FaultContext = createContext<WordImport | null>(null);

/**
 * ImportWorkspacePage is one import's page, from the draft at
 * /teacher/imports/new to the review it hands over to, as the approved
 * page-width design draws it (DG-206): the Sources pane beside the status
 * pane, stacked below 960px of content, with the status pane first once
 * processing has started. Starting a draft replaces the address with the
 * import's own, /teacher/imports/:id, which stays the same through every
 * state; a committing or committed import is shown by its confirm page.
 */
export default function ImportWorkspacePage() {
  const { id } = useParams();
  return id === undefined ? <DraftWorkspace /> : <ImportWorkspace id={id} />;
}

function DraftWorkspace() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const availability = useImportAvailability();
  const processing = availability === "on";
  const retention = useImportRetention();
  const [params, setParams] = useSearchParams();
  const pasting = params.get("source") === "paste";
  const head = (
    <PageHead
      title={pasting ? t("imports.paste.title") : t("imports.newTitle")}
      description={pasting ? t("imports.paste.sub") : t("imports.newHint")}
      crumb={pasting}
      back={{ to: "/teacher/imports", label: t("imports.historyTitle") }}
      actions={
        availability === "reviewOnly" || availability === "off" ? undefined : (
          <Segmented
            size="lg"
            scroll
            className="[&>button]:min-w-[150px] [&>button]:justify-center"
            label={t("imports.upload.source")}
            value={pasting ? "paste" : "file"}
            options={[
              { value: "file", label: t("imports.upload.fileMode"), icon: FileUp },
              {
                value: "paste",
                label: t("imports.upload.pasteMode"),
                icon: ClipboardPaste,
              },
            ]}
            onChange={(value) =>
              setParams(
                (current) => {
                  const next = new URLSearchParams(current);
                  if (value === "paste") next.set("source", "paste");
                  else next.delete("source");
                  return next;
                },
                { replace: true },
              )
            }
          />
        )
      }
    />
  );
  if (availability === "reviewOnly" || availability === "off")
    return (
      <div className="flex w-full min-w-0 flex-col gap-4.5" data-scale="deck">
        {head}
        <EmptyState
          hint={t("imports.availability.newOffHint")}
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
            </Button>
          }
        >
          {t("imports.availability.newOff")}
        </EmptyState>
      </div>
    );
  return (
    <div className="flex w-full min-w-0 flex-col gap-4.5" data-scale="deck">
      {head}
      <SourceIntake
        existing={null}
        fileMode={!pasting}
        enabled={processing}
        onStarted={(started) => {
          void storeImport(client, started);
          void navigate(`/teacher/imports/${started.id}`, { replace: true });
        }}
      >
        {(parts) => (
          <WorkspaceGrid
            statusFirst={false}
            sources={
              <Pane
                title={t("imports.upload.source")}
                aside={parts.pasting ? undefined : <FormatsLine />}
              >
                <PaneBody>
                  {parts.sources}
                  <Privacy processing={processing} retention={retention} />
                </PaneBody>
              </Pane>
            }
            status={<DraftStatus parts={parts} />}
          />
        )}
      </SourceIntake>
    </div>
  );
}

function DraftStatus({
  parts,
  notices,
  intro,
  closing,
}: Readonly<{
  parts: IntakeParts;
  notices?: ReactNode;
  intro?: ReactNode;
  closing?: ReactNode;
}>) {
  const { t } = useTranslation();
  return (
    <Pane
      title={
        parts.pasting ? t("imports.workspace.readAs") : t("imports.workspace.steps")
      }
    >
      {notices}
      {parts.pasting ? parts.findings : <ProcessingPanel run={undefined} />}
      <PaneBody ruled>
        {intro}
        {parts.details}
      </PaneBody>
      <PaneBody ruled>
        {parts.footer}
        {closing}
      </PaneBody>
    </Pane>
  );
}

function FormatsLine() {
  const { t } = useTranslation();
  const limits = useQuery(importLimitsQuery());
  if (limits.data === undefined) return null;
  return t("imports.newFormats", {
    formats: listFormats(limits.data.formats, "name", "or", t),
    size: formatBytes(limits.data.maxBytes),
  });
}

function Privacy({
  processing,
  retention,
}: Readonly<{ processing: boolean; retention: ImportRetention | undefined }>) {
  const { t } = useTranslation();
  return (
    <>
      <Callout
        icon={ShieldCheck}
        lead={t("imports.upload.privacyLead")}
        className="gap-3 [&>svg]:size-[17px]"
      >
        {t("imports.upload.privacy", {
          processing: processing
            ? t("imports.upload.processingOn")
            : t("imports.upload.processingUnknown"),
        })}
      </Callout>
      {retention === undefined ? null : (
        <p className="text-muted-fg text-xs leading-relaxed">
          {t("imports.retention.policy", {
            afterCommit: retention.afterCommitDays,
            afterCancel: retention.afterCancelDays,
            idle: retention.idleDays,
          })}
        </p>
      )}
    </>
  );
}

function ImportWorkspace({ id }: Readonly<{ id: string }>) {
  const { t } = useTranslation();
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
  useCrumbs(query.data ? [{ label: importName(query.data, t) }] : null);

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
          className="flex w-full flex-col gap-3 focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!"
        >
          <Skeleton className="h-8 w-72" />
          <div className="@container/workspace">
            <div className="grid gap-4 @[960px]/workspace:grid-cols-(--pane-split)">
              <Skeleton className="h-60 w-full" />
              <Skeleton className="h-60 w-full" />
            </div>
          </div>
        </div>
      );
    if (query.error instanceof ApiError && query.error.status === 404)
      return (
        <div className="w-full min-w-0" data-scale="deck">
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
      <div className="w-full min-w-0" data-scale="deck">
        <LoadError error={query.error} onRetry={() => void query.refetch()}>
          {t("imports.detailFailed")}
        </LoadError>
      </div>
    );
  }
  const value = query.data;
  if (showsOnConfirm(value))
    return <Navigate replace to={`/teacher/imports/${value.id}/confirm`} />;
  const outcome = reprocessOutcome(value);
  const notices = (
    <>
      {query.isError ? (
        <div className="border-b px-4.5 py-3">
          <StaleNotice onRetry={() => void query.refetch()} />
        </div>
      ) : null}
      {value.filesRemovedAt === undefined ? null : (
        <div className="border-b px-4.5 py-3">
          <FilesRemovedNotice at={value.filesRemovedAt} />
        </div>
      )}
    </>
  );
  return (
    <div className="flex w-full min-w-0 flex-col gap-4.5" data-scale="deck">
      <ImportHead value={value} />
      <p role="status" aria-live="polite" className="sr-only">
        {outcome === null
          ? t(`imports.status.${value.status}`)
          : t(`imports.reprocess.${outcome}`)}
      </p>
      <StateWorkspace
        value={value}
        onChange={store}
        processing={processing}
        notices={notices}
      />
    </div>
  );
}

function showsOnConfirm(value: WordImport): boolean {
  if (value.status === "committed") return value.testId !== undefined;
  return value.status === "committing" && value.filesRemovedAt === undefined;
}

function ImportHead({ value }: Readonly<{ value: WordImport }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const key = value.sources.find((source) => source.role === "answer_key");
  const started = value.run?.createdAt ?? value.createdAt;
  const text = isTextImport(value);
  const name = importName(value, t);
  return (
    <PageHead
      title={name}
      back={{ to: "/teacher/imports", label: t("imports.historyTitle") }}
      status={
        value.status === "awaiting_sources" ? undefined : (
          <ImportStatusBadge status={value.status} />
        )
      }
    >
      <p
        className="text-muted-fg text-sm break-words"
        title={formatDateTime(started, locale)}
      >
        {t(
          value.run === undefined ? "imports.detail.created" : "imports.detail.started",
          {
            when: formatRelative(started, locale),
          },
        )}
        {text ? <> · {t("imports.detail.fromPastedText")}</> : null}
        {key ? <> · {t("imports.history.withKey", { name: key.filename })}</> : null}
      </p>
      {name === value.title ? null : (
        <p className="text-muted-fg text-xs break-words">{value.title}</p>
      )}
    </PageHead>
  );
}

function StateWorkspace({
  value,
  onChange,
  processing,
  notices,
}: Readonly<{
  value: WordImport;
  onChange: Store;
  processing: boolean;
  notices: ReactNode;
}>) {
  const intake =
    processing &&
    !isTextImport(value) &&
    (value.status === "awaiting_sources" || value.status === "failed");
  if (intake)
    return <FileWorkspace value={value} onChange={onChange} notices={notices} />;
  const started = value.status !== "awaiting_sources";
  return (
    <WorkspaceGrid
      statusFirst={started}
      sources={<SourcesPane value={value} />}
      status={
        <StatusPane
          value={value}
          onChange={onChange}
          processing={processing}
          notices={notices}
        />
      }
    />
  );
}

function SourcesPane({ value }: Readonly<{ value: WordImport }>) {
  const { t } = useTranslation();
  return (
    <Pane title={t("imports.upload.source")}>
      <PaneBody>
        <SourcesList
          importId={value.id}
          sources={value.sources}
          pendingUploads={value.pendingUploads}
          removed={value.filesRemovedAt !== undefined}
          pillOf={(source) => pillOf(value, source)}
        />
      </PaneBody>
    </Pane>
  );
}

function pillOf(value: WordImport, source: ImportSource): SourcePill | null {
  switch (value.status) {
    case "queued":
      return "waiting";
    case "processing":
      return "reading";
    case "needs_review":
      return "read";
    case "failed":
      return source.role === "exam" && !isRetryable(value.run?.errorCode)
        ? "failed"
        : null;
    default:
      return null;
  }
}

function FileWorkspace({
  value,
  onChange,
  notices,
}: Readonly<{ value: WordImport; onChange: Store; notices: ReactNode }>) {
  const { t } = useTranslation();
  const failed = value.status === "failed";
  const fileFault = failed && !isRetryable(value.run?.errorCode);
  return (
    <FaultContext value={value}>
      <SourceIntake
        key={value.id}
        existing={value}
        replacing={failed}
        examAlert={fileFault ? ExamFault : undefined}
        onChanged={(next) => void onChange(next)}
        onStarted={(next) => void onChange(next)}
      >
        {(parts) => (
          <WorkspaceGrid
            statusFirst={failed}
            sources={
              <Pane title={t("imports.upload.source")}>
                <PaneBody>{parts.sources}</PaneBody>
              </Pane>
            }
            status={
              failed ? (
                <Pane title={t("imports.workspace.progress")}>
                  {notices}
                  <ProcessingPanel
                    run={value.run}
                    exam={value.sources.find((source) => source.role === "exam")}
                    failed
                  />
                  <PaneBody ruled>
                    <Stopped
                      value={value}
                      onChange={onChange}
                      canRetry
                      fileFault={fileFault}
                    />
                  </PaneBody>
                  <PaneBody ruled>
                    {parts.footer}
                    <CloseImport value={value} onChange={onChange} />
                    <IdleWarning />
                  </PaneBody>
                </Pane>
              ) : (
                <DraftStatus
                  parts={parts}
                  notices={notices}
                  intro={
                    <>
                      <h3 className="text-ui font-semibold">
                        {t("imports.detail.awaitingTitle")}
                      </h3>
                      <p className="text-muted-fg text-sm">
                        {t("imports.detail.awaitingBody")}
                      </p>
                    </>
                  }
                  closing={
                    <>
                      <CloseImport value={value} onChange={onChange} />
                      <IdleWarning />
                    </>
                  }
                />
              )
            }
          />
        )}
      </SourceIntake>
    </FaultContext>
  );
}

function ExamFault({ onChoose }: Readonly<{ onChoose: () => void }>) {
  const { t } = useTranslation();
  const value = useContext(FaultContext);
  if (value === null) return null;
  return (
    <Callout
      tone="danger"
      size="md"
      actions={
        <Button type="button" size="sm" onClick={onChoose}>
          {t("imports.detail.replaceExam")}
        </Button>
      }
    >
      {failureDescription(value, t)}
    </Callout>
  );
}

function Stopped({
  value,
  onChange,
  canRetry,
  fileFault = false,
}: Readonly<{
  value: WordImport;
  onChange: Store;
  canRetry: boolean;
  fileFault?: boolean;
}>) {
  const { t } = useTranslation();
  return (
    <div role="alert" className="flex flex-col gap-3">
      <FailureStage value={value} />
      <Callout tone="danger">
        <NoteText
          title={failureTitle(value, t)}
          body={
            fileFault ? t("imports.detail.replaceHint") : failureDescription(value, t)
          }
        />
      </Callout>
      <RetryRun value={value} onChange={onChange} canRetry={canRetry} />
    </div>
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
      className="h-6.5! w-fit self-start rounded-full! text-[12.5px] whitespace-normal"
    >
      <CircleX className="size-[13px]" aria-hidden="true" />
      {t("imports.detail.stoppedAt", { stage: label })}
    </Badge>
  );
}

function StatusPane({
  value,
  onChange,
  processing,
  notices,
}: Readonly<{
  value: WordImport;
  onChange: Store;
  processing: boolean;
  notices: ReactNode;
}>) {
  const { t } = useTranslation();
  const exam = value.sources.find((source) => source.role === "exam");
  switch (value.status) {
    case "awaiting_sources":
      return (
        <Pane title={t("imports.workspace.steps")}>
          {notices}
          <ProcessingPanel run={undefined} exam={exam} />
          <PaneBody ruled>
            <AwaitingActions
              value={value}
              onChange={onChange}
              processing={processing}
            />
          </PaneBody>
        </Pane>
      );
    case "failed":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <ProcessingPanel run={value.run} exam={exam} failed />
          <PaneBody ruled>
            <Stopped value={value} onChange={onChange} canRetry={processing} />
            <FailedActions value={value} onChange={onChange} processing={processing} />
          </PaneBody>
        </Pane>
      );
    case "queued":
    case "processing":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <ProcessingPanel run={value.run} exam={exam} />
          <PaneBody ruled>
            <RunningActions value={value} onChange={onChange} />
          </PaneBody>
        </Pane>
      );
    case "needs_review":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <ProcessingPanel
            run={value.run}
            exam={exam}
            failed={value.run?.status === "failed"}
          />
          <PaneBody ruled>
            <Ready value={value} onChange={onChange} processing={processing} />
          </PaneBody>
        </Pane>
      );
    case "cancelled":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <PaneBody>
            <Closed value={value} processing={processing} />
          </PaneBody>
        </Pane>
      );
    case "committing":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <PaneBody>
            <Callout>
              <NoteText
                title={t("imports.detail.committingTitle")}
                body={t("imports.detail.committingBody")}
              />
            </Callout>
          </PaneBody>
        </Pane>
      );
    case "committed":
      return (
        <Pane title={t("imports.workspace.progress")}>
          {notices}
          <PaneBody>
            <Callout tone="success" icon={CircleCheck}>
              <NoteText
                title={t("imports.detail.committedTitle")}
                body={t("imports.detail.committedBody")}
              />
            </Callout>
            {value.testId === undefined ? null : (
              <Button asChild className="self-start">
                <Link to={`/teacher/tests/${value.testId}/edit`}>
                  {t("imports.detail.openBuilder")}
                </Link>
              </Button>
            )}
          </PaneBody>
        </Pane>
      );
  }
}

function NoteText({
  title,
  body,
}: Readonly<{ title: string; body?: string | undefined }>) {
  return (
    <>
      <span className="block font-semibold">{title}</span>
      {body === undefined ? null : <span className="block">{body}</span>}
    </>
  );
}

function AwaitingActions({
  value,
  onChange,
  processing,
}: Readonly<{ value: WordImport; onChange: Store; processing: boolean }>) {
  const { t } = useTranslation();
  const text = isTextImport(value);
  return (
    <>
      <h3 className="text-ui font-semibold">{t("imports.detail.awaitingTitle")}</h3>
      {processing ? (
        <p className="text-muted-fg text-sm">
          {t(text ? "imports.detail.text.awaitingBody" : "imports.detail.awaitingBody")}
        </p>
      ) : (
        <ProcessingOffNotice>{t("imports.availability.intakeOff")}</ProcessingOffNotice>
      )}
      {processing && text ? (
        <RetryRun
          value={value}
          onChange={onChange}
          canRetry
          label={t("imports.upload.start")}
        />
      ) : null}
      <CloseImport value={value} onChange={onChange} />
      <IdleWarning />
    </>
  );
}

function FailedActions({
  value,
  onChange,
  processing,
}: Readonly<{ value: WordImport; onChange: Store; processing: boolean }>) {
  const { t } = useTranslation();
  return (
    <>
      {processing ? null : (
        <ProcessingOffNotice>{t("imports.availability.intakeOff")}</ProcessingOffNotice>
      )}
      {processing && isTextImport(value) ? (
        <Button
          asChild
          size="md"
          className="h-10 self-start px-4 text-[14px] font-semibold"
        >
          <Link to="/teacher/imports/new?source=paste">
            {t("imports.detail.startOver")}
          </Link>
        </Button>
      ) : null}
      <CloseImport value={value} onChange={onChange} />
      <IdleWarning />
    </>
  );
}

function RunningActions({
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
    <>
      <h3 className="sr-only">
        {reprocess
          ? t("imports.detail.reprocessTitle")
          : t("imports.detail.processingTitle")}
      </h3>
      <p className="text-muted-fg text-sm">
        {waitingForWorker ? t("imports.detail.waitingLeave") : body}
      </p>
      {waitingForWorker ? (
        <ProcessingOffNotice>{t("imports.availability.queuedOff")}</ProcessingOffNotice>
      ) : null}
      {cancel.error === null ? null : (
        <p role="alert" className="text-sm">
          {cancel.error}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-end gap-2">
        {reprocess ? (
          <Button asChild variant="outline" size="md">
            <Link to={`/teacher/imports/${value.id}/review`}>
              {t("imports.detail.viewReview")}
            </Link>
          </Button>
        ) : null}
        <Button
          variant="outline"
          size="md"
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
    </>
  );
}

function Ready({
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
    <>
      <Callout tone="success" icon={CircleCheck}>
        <NoteText
          title={
            review.data === undefined
              ? t("imports.detail.readyTitle")
              : readyTitle(review.data.summary, t)
          }
          body={t("imports.detail.readyBody")}
        />
      </Callout>
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
      <div className="flex flex-wrap items-center justify-end gap-2">
        <CloseImport value={value} onChange={onChange} />
        <Button asChild variant="outline" size="md" className="h-10">
          <Link to="/teacher/imports">{t("imports.detail.later")}</Link>
        </Button>
        <Button asChild size="md" className="h-10 px-4 text-[14px] font-semibold">
          <Link to={`/teacher/imports/${value.id}/review`}>
            {t("imports.detail.startReview")}
            <ArrowRight aria-hidden="true" />
          </Link>
        </Button>
      </div>
      <IdleWarning />
    </>
  );
}

function Closed({
  value,
  processing,
}: Readonly<{ value: WordImport; processing: boolean }>) {
  const { t } = useTranslation();
  const retention = useImportRetention();
  const processingOff = useProcessingOff();
  const removed = value.filesRemovedAt !== undefined;
  return (
    <>
      <Callout>
        <NoteText
          title={t("imports.detail.cancelledTitle")}
          body={closedBody(t, value, retention)}
        />
      </Callout>
      {processingOff ? (
        <ProcessingOffNotice>
          {t("imports.availability.startOverOff")}
        </ProcessingOffNotice>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
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
        {hasDraft(value) && !removed ? (
          <Button asChild variant="outline">
            <Link to={`/teacher/imports/${value.id}/review`}>
              {t("imports.detail.viewReview")}
            </Link>
          </Button>
        ) : null}
      </div>
    </>
  );
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
        <Badge variant="success" className="h-6.5! rounded-full! text-[12.5px]">
          <CircleCheck className="size-[13px]" aria-hidden="true" />
          {t("imports.detail.everyAnswer")}
        </Badge>
      ) : null}
      {summary.blocking > 0 ? (
        <Badge variant="danger" className="h-6.5! rounded-full! text-[12.5px]">
          <CircleAlert className="size-[13px]" aria-hidden="true" />
          {t("imports.detail.needAction", { count: summary.blocking })}
        </Badge>
      ) : null}
      {summary.needsDecision > 0 ? (
        <Badge variant="warning" className="h-6.5! rounded-full! text-[12.5px]">
          <CircleHelp className="size-[13px]" aria-hidden="true" />
          {t("imports.detail.toConfirm", { count: summary.needsDecision })}
        </Badge>
      ) : null}
      {notes > 0 ? (
        <Badge variant="secondary" className="h-6.5! rounded-full! text-[12.5px]">
          <Info className="size-[13px]" aria-hidden="true" />
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
        size={
          value.status === "needs_review" || value.status === "failed"
            ? "md"
            : "default"
        }
        className={cn(
          "text-muted-foreground self-start",
          (value.status === "needs_review" || value.status === "failed") && "h-10",
        )}
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
