import {
  useCallback,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { useLocale } from "@/lib/i18n/useLocale";
import {
  Link,
  useBeforeUnload,
  useBlocker,
  useLocation,
  useNavigate,
  useParams,
} from "react-router";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  CircleStop,
  Clock,
  Copy,
  Ellipsis,
  Pencil,
  RotateCcw,
  Send,
  SquarePen,
} from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { can, hasWorkspace, useCan, useWorkspace } from "@/features/auth/permissions";
import {
  getMonitor,
  setAttemptNote,
  type AttemptReview,
  type MonitorRow,
} from "@/features/attempts/api";
import { AttemptSheet } from "@/features/attempts/components/AttemptSheet";
import { Monitor } from "@/features/attempts/components/Monitor";
import { ProgressCard } from "@/features/attempts/components/ProgressCard";
import { SheetNoteController } from "@/features/attempts/components/sheetNotes";
import { POLL_MS, monitorKey, reviewKey } from "@/features/attempts/keys";
import { listVersions, previewTest } from "@/features/tests/api";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { PageHead } from "@/layouts/shell/PageHead";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { ApiError } from "@/lib/api/errors";
import { formatMoment, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";
import { getAssignment, updateAssignment, type Assignment } from "../../api";
import { useAssignmentInsights } from "../../assignmentInsights";
import { AiInsightsAction, AiInsightsPanel } from "../../components/AiInsightsPanel";
import { CloseEarlyDialog } from "../../components/CloseEarlyDialog";
import { ItemAnalysis } from "../../components/ItemAnalysis";
import { ReopenStudentDialog } from "../../components/ReopenStudentDialog";
import { SettingsGroups } from "../../components/SettingsGroups";
import { keepsTimeAfter, overridesKey } from "../../overrides";
import { toInput } from "../../input";
import { statusAt } from "../../status";
import {
  PROGRESS_GROUPS,
  assignmentStats,
  detailTab,
  firstPendingPaper,
  rosterFilter,
  type ProgressGroup,
} from "./assignmentDetail";
import { assignmentDetailLocation } from "./assignmentDetailUrl";
import { moment } from "./assignmentWindow";
import { DuplicateAssignmentDialog } from "./DuplicateAssignmentDialog";
import { ExtendAssignmentsDialog } from "./ExtendAssignmentsDialog";

/** AssignmentDetailPage owns the assignment monitor and preserves authorized actions and note drafts across its URL-controlled tabs and sheet. */
export default function AssignmentDetailPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const params = new URLSearchParams(location.search);
  const tab = detailTab(params.get("tab"));
  const roster = rosterFilter(params.get("roster"));
  const selectedGroup = PROGRESS_GROUPS.find((group) => group === roster) ?? null;
  const attemptId = params.get("attempt");
  const workspace = useWorkspace("teacher");
  const write = useCan("teaching.assignments.write");
  const client = useQueryClient();
  const [dialog, setDialog] = useState<DetailDialog | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const moreRef = useRef<HTMLButtonElement>(null);
  const extendRef = useRef<HTMLButtonElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const headingAfterPublish = useRef(false);
  const { notes, recoveryPanel, noteFocusRequest } = useSheetDrafts();
  const insights = useAssignmentInsights(id);

  const { assignment, a, now, status, monitor, version, preview } = useAssignmentReads(
    id,
    workspace,
    tab,
  );
  const [sheetIdentity, setSheetIdentity] = useState<{
    assignmentId: string;
    attemptId: string;
  } | null>(null);
  const hostReady =
    workspace && assignment.isSuccess && a !== undefined && status !== null;
  if (!hostReady && sheetIdentity !== null) setSheetIdentity(null);
  else if (
    hostReady &&
    attemptId &&
    (sheetIdentity?.assignmentId !== id || sheetIdentity.attemptId !== attemptId)
  )
    setSheetIdentity({ assignmentId: id, attemptId });
  else if (!attemptId && sheetIdentity !== null && sheetIdentity.assignmentId !== id)
    setSheetIdentity(null);
  const sheetAttempt =
    attemptId || (sheetIdentity?.assignmentId === id ? sheetIdentity.attemptId : null);
  const monitorRows = monitor.isSuccess ? monitor.data.rows : undefined;
  const stats = useMemo(
    () => (monitorRows === undefined ? null : assignmentStats(monitorRows)),
    [monitorRows],
  );
  useCrumbs(a === undefined ? null : [{ label: a.testTitle }]);
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["teacher-assignment", id] }),
      client.invalidateQueries({ queryKey: ["teacher-assignments"] }),
      client.invalidateQueries({ queryKey: ["teacher-dashboard"] }),
      client.invalidateQueries({ queryKey: monitorKey(id) }),
      client.invalidateQueries({ queryKey: overridesKey(id) }),
    ]);
  };
  const publish = useMutation({
    mutationFn: (value: Assignment) => {
      if (!write || value.id !== id)
        throw new Error("Assignment write is not permitted");
      return updateAssignment(value.id, { ...toInput(value), draft: false });
    },
    onSuccess: async () => {
      headingAfterPublish.current = true;
      await refresh();
    },
    onError: (error) =>
      setFailure(
        error instanceof ApiError
          ? error.message
          : t("assignments.detail.publishFailed"),
      ),
  });
  useEffect(() => {
    if (!headingAfterPublish.current || status === null || status === "draft") return;
    headingAfterPublish.current = false;
    if (document.activeElement === null || document.activeElement === document.body)
      headingRef.current?.focus();
  }, [status]);
  const close = useMutation({
    mutationFn: (value: Assignment) => {
      if (!write || value.id !== id || statusAt(value, new Date()) !== "open")
        throw new Error("Assignment cannot close");
      return updateAssignment(value.id, {
        ...toInput(value),
        draft: false,
        closeNow: true,
      });
    },
    onSuccess: async () => {
      setDialog(null);
      await refresh();
    },
  });
  if (!workspace)
    return (
      <>
        {recoveryPanel}
        <EmptyState>{t("assignmentDetail.unavailable")}</EmptyState>
      </>
    );
  if (assignment.isPending)
    return (
      <>
        {recoveryPanel}
        <ListSkeleton rows={8} />
      </>
    );
  if (assignment.isError)
    return (
      <>
        {recoveryPanel}
        <LoadError error={assignment.error} onRetry={() => void assignment.refetch()}>
          {t("assignments.detail.loadFailed")}
        </LoadError>
      </>
    );
  if (a === undefined || status === null)
    return (
      <>
        {recoveryPanel}
        <EmptyState
          action={
            <Button variant="outline" asChild>
              <Link to="/teacher/assignments">
                {t("assignments.detail.backToList")}
              </Link>
            </Button>
          }
        >
          {t("assignments.detail.notFound")}
        </EmptyState>
      </>
    );
  const eligible =
    monitorRows === undefined ? undefined : firstPendingPaper(monitorRows);
  const openAttempt = (selected: string) =>
    navigate(assignmentDetailLocation(location, { attempt: selected }));
  const showGroup = (group: ProgressGroup | null) =>
    void navigate(
      assignmentDetailLocation(location, {
        tab: "students",
        roster: group,
        page: null,
      }),
      { replace: true },
    );
  return (
    <div className="flex min-w-0 flex-col gap-4.5">
      <PageHead
        title={a.testTitle}
        headingRef={headingRef}
        status={<StatusBadge kind="assignment" status={status} />}
        className="items-start! [&_h1]:text-balance"
        actions={
          <>
            <WriteActions
              a={a}
              status={status}
              pending={publish.isPending}
              moreRef={moreRef}
              extendRef={extendRef}
              onPublish={() => publish.mutate(a)}
              onDialog={setDialog}
              onEditSettings={() =>
                void navigate(assignmentDetailLocation(location, { tab: "settings" }), {
                  replace: true,
                })
              }
            />
            <GradeAction
              count={stats?.pending ?? 0}
              attemptId={eligible?.attemptId}
              onOpen={(selected) => void openAttempt(selected)}
            />
          </>
        }
      >
        <AssignmentMeta a={a} status={status} />
      </PageHead>
      {failure !== null && (
        <p role="alert" className="text-danger-ink text-sm">
          {failure}
        </p>
      )}
      <DraftHint a={a} status={status} />
      {recoveryPanel !== null && !attemptId && recoveryPanel}
      <DetailProgress
        monitor={monitor}
        stats={stats}
        selected={selectedGroup}
        onSelect={showGroup}
      />
      <Tabs
        value={tab}
        onValueChange={(value) =>
          void navigate(assignmentDetailLocation(location, { tab: value }), {
            replace: true,
          })
        }
      >
        <TabsList aria-label={t("assignmentDetail.tabsLabel")}>
          {(["students", "questions", "settings"] as const).map((value) => (
            <TabsTrigger key={value} value={value}>
              {t(`assignmentDetail.tabs.${value}`)}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="students" className="pt-4.5">
          <StudentsPanel
            monitor={monitor}
            a={a}
            attemptId={attemptId}
            onOpen={(selected) => void openAttempt(selected)}
            onRefresh={refresh}
          />
        </TabsContent>
        <TabsContent value="questions" className="space-y-4 pt-4.5">
          <ItemAnalysis
            assignmentId={a.id}
            action={(handedIn) => (
              <AiInsightsAction insights={insights} handedIn={handedIn} />
            )}
          >
            <AiInsightsPanel
              insights={insights}
              assignmentId={a.id}
              testTitle={a.testTitle}
              groups={preview.data?.groups}
            />
          </ItemAnalysis>
        </TabsContent>
        <TabsContent value="settings" className="pt-4.5">
          <SettingsGroups a={a} version={version} status={status} />
        </TabsContent>
      </Tabs>
      <StateDialogs
        a={a}
        status={status}
        dialog={dialog}
        pending={close.isPending}
        failed={close.isError}
        rows={monitor.data?.rows ?? null}
        now={now}
        moreRef={moreRef}
        extendRef={extendRef}
        onDialog={setDialog}
        onClose={() => {
          if (write && status === "open") close.mutate(a);
        }}
        onRefresh={refresh}
      />
      {sheetAttempt && (
        <AttemptSheet
          assignment={a}
          open={Boolean(attemptId)}
          attemptId={sheetAttempt}
          row={monitor.data?.rows.find((row) => row.attemptId === sheetAttempt)}
          questionCount={monitor.data?.questionCount ?? 0}
          serverTime={
            monitor.data?.serverTime ?? new Date(monitor.dataUpdatedAt).toISOString()
          }
          receivedAt={monitor.dataUpdatedAt}
          notes={notes}
          noteFocusRequest={noteFocusRequest}
          onClose={() => {
            void navigate(assignmentDetailLocation(location, { attempt: null }), {
              replace: true,
            });
          }}
          recovery={recoveryPanel}
        />
      )}
    </div>
  );
}

function useAssignmentReads(
  id: string,
  workspace: boolean,
  tab: ReturnType<typeof detailTab>,
) {
  const assignment = useQuery({
    queryKey: ["teacher-assignment", id],
    queryFn: ({ signal }) => getAssignment(id, signal),
    enabled: workspace && id !== "",
  });
  const a = workspace && assignment.data?.id === id ? assignment.data : undefined;
  const now = new Date();
  const status = a === undefined ? null : statusAt(a, now);
  const live = status === "open" && assignment.isSuccess;
  const interval = useIdlePolling(POLL_MS, live);
  const monitor = useQuery({
    queryKey: monitorKey(id),
    queryFn: ({ signal }) => getMonitor(id, signal),
    enabled: a !== undefined && assignment.isSuccess,
    refetchInterval: (query) => (query.state.status === "error" ? false : interval),
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(monitor.refetch, live && monitor.isSuccess);
  const testId = a?.testId;
  const versions = useQuery({
    queryKey: ["teacher-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: a !== undefined && testId !== undefined,
  });
  const version = versions.data?.items.find((v) => v.id === a?.testVersionId);
  const preview = useQuery({
    queryKey: ["teacher-test-preview", testId, a?.testVersion],
    queryFn: ({ signal }) => previewTest(testId ?? "", a?.testVersion, signal),
    enabled: a !== undefined && tab === "questions",
  });
  return { assignment, a, now, status, monitor, version, preview };
}

function AssignmentMeta({
  a,
  status,
}: Readonly<{ a: Assignment; status: ReturnType<typeof statusAt> }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <p className="text-muted-fg text-ui mt-1">
      {a.targets.classes.map((value) => value.name).join(", ")} ·{" "}
      {formatMoment(
        status === "scheduled"
          ? a.window.opensAt
          : (a.window.closedAt ?? a.window.closesAt),
        locale,
      )}{" "}
      · {t("assignments.minutes", { count: a.durationMinutes })}
    </p>
  );
}

function DraftHint({
  a,
  status,
}: Readonly<{ a: Assignment; status: ReturnType<typeof statusAt> }>) {
  const { t } = useTranslation();
  if (status !== "draft") return null;
  return (
    <p className="text-muted-fg text-sm">
      {t("assignmentDetail.draftHint")}
      {(a.targetCount ?? 0) === 0 ? ` ${t("assignments.needTargets")}` : ""}
    </p>
  );
}

function useSheetDrafts() {
  const client = useQueryClient();
  const [recovery, setRecovery] = useState(false);
  const [noteFocusRequest, setNoteFocusRequest] = useState(0);
  const [resolution, setResolution] = useState<"save" | "discard">("save");
  const [retry, setRetry] = useState(0);
  const [notes] = useState(
    () =>
      new SheetNoteController(async (noteId, value) => {
        const user = useAuthStore.getState().user;
        if (!hasWorkspace(user, "teacher") || !can(user, "teaching.grading"))
          throw new Error("Note write is not permitted");
        const result = await setAttemptNote(noteId, value);
        client.setQueryData<AttemptReview>(reviewKey(noteId), (data) =>
          data?.attempt.id === noteId ? { ...data, teacherNote: value } : data,
        );
        return result;
      }),
  );
  useSyncExternalStore(notes.subscribe, notes.snapshot, notes.snapshot);
  const blocker = useBlocker(notes.unsettled);
  const blockedKey = blocker.state === "blocked" ? blocker.location.key : null;
  const departedKey = useRef<string | null>(null);
  const depart = useEffectEvent((key: string, saved: boolean) => {
    if (
      blocker.state === "blocked" &&
      blocker.location.key === key &&
      departedKey.current !== key
    ) {
      if (!saved) {
        setNoteFocusRequest((value) => value + 1);
        setRecovery(true);
        return;
      }
      departedKey.current = key;
      setRecovery(false);
      setResolution("save");
      blocker.proceed();
    }
  });
  useEffect(() => {
    let active = true;
    if (blockedKey === null) departedKey.current = null;
    if (blockedKey !== null)
      void (
        resolution === "discard"
          ? notes.discardAll().then(() => true)
          : notes.flushAll()
      ).then((saved) => {
        if (!active) return;
        depart(blockedKey, saved);
      });
    return () => {
      active = false;
    };
  }, [blockedKey, notes, resolution, retry]);
  useBeforeUnload(
    useCallback(
      (event) => {
        if (notes.unsettled) event.preventDefault();
      },
      [notes],
    ),
  );

  const recoveryPanel = recovery ? (
    <NoteRecovery
      onRetry={() => {
        setResolution("save");
        setRetry((value) => value + 1);
      }}
      onDiscard={() => {
        setResolution("discard");
        setRetry((value) => value + 1);
      }}
      onStay={() => {
        if (blocker.state === "blocked") {
          blocker.reset();
        }
        setRecovery(false);
        setResolution("save");
      }}
    />
  ) : null;
  return { notes, recoveryPanel, noteFocusRequest };
}

type DetailDialog = { kind: "close" | "reopenStudent" | "extend" | "duplicate" };

function StateDialogs({
  a,
  status,
  dialog,
  pending,
  failed,
  rows,
  now,
  moreRef,
  extendRef,
  onDialog,
  onClose,
  onRefresh,
}: Readonly<{
  a: Assignment;
  status: ReturnType<typeof statusAt>;
  dialog: DetailDialog | null;
  pending: boolean;
  failed: boolean;
  rows: readonly MonitorRow[] | null;
  now: Date;
  moreRef: RefObject<HTMLButtonElement | null>;
  extendRef: RefObject<HTMLButtonElement | null>;
  onDialog: (value: DetailDialog | null) => void;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const write = useCan("teaching.assignments.write");
  const intervene = useCan("teaching.attempts.intervene");
  const kind = dialog?.kind;
  const closeUnless = (open: boolean) => {
    if (!open) onDialog(null);
  };
  return (
    <>
      {write && status === "open" && (
        <CloseEarlyDialog
          assignment={a}
          open={kind === "close"}
          pending={pending}
          failed={failed}
          keepTime={rows === null ? 0 : keepsTimeAfter(rows, now)}
          returnFocus={moreRef}
          onOpenChange={(open) => onDialog(open ? { kind: "close" } : null)}
          onConfirm={onClose}
        />
      )}
      {intervene && status !== "draft" && rows !== null && (
        <ReopenStudentDialog
          assignment={a}
          rows={rows}
          open={kind === "reopenStudent"}
          returnFocus={moreRef}
          onOpenChange={closeUnless}
          onDone={onRefresh}
        />
      )}
      {write && status !== "draft" && (
        <ExtendAssignmentsDialog
          items={[a]}
          when={moment(a.window.closedAt ?? a.window.closesAt, now, t, true)}
          audience={intervene && rows !== null ? rows : undefined}
          open={kind === "extend"}
          returnFocus={extendRef}
          onOpenChange={closeUnless}
          onExtended={() => void onRefresh()}
        />
      )}
      {write && (
        <DuplicateAssignmentDialog
          assignment={a}
          open={kind === "duplicate"}
          returnFocus={moreRef}
          onOpenChange={closeUnless}
          onDuplicated={(draft) => {
            void onRefresh();
            toast(t("assignments.list.duplicated"), {
              action: {
                label: t("assignments.list.open"),
                onClick: () => void navigate(`/teacher/assignments/${draft.id}`),
              },
            });
          }}
        />
      )}
    </>
  );
}

function NoteRecovery({
  onRetry,
  onDiscard,
  onStay,
}: Readonly<{ onRetry: () => void; onDiscard: () => void; onStay: () => void }>) {
  const { t } = useTranslation();
  return (
    <div role="alert" className="bg-danger-soft rounded-ctl space-y-2 p-3 text-sm">
      <p>{t("assignmentDetail.sheet.leaveFailed")}</p>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
        <Button size="sm" variant="outline" onClick={onDiscard}>
          {t("assignmentDetail.sheet.discardLeave")}
        </Button>
        <Button size="sm" variant="ghost" onClick={onStay}>
          {t("assignmentDetail.sheet.stay")}
        </Button>
      </div>
    </div>
  );
}

function WriteActions({
  a,
  status,
  pending,
  moreRef,
  extendRef,
  onPublish,
  onDialog,
  onEditSettings,
}: Readonly<{
  a: Assignment;
  status: ReturnType<typeof statusAt>;
  pending: boolean;
  moreRef: RefObject<HTMLButtonElement | null>;
  extendRef: RefObject<HTMLButtonElement | null>;
  onPublish: () => void;
  onDialog: (dialog: DetailDialog) => void;
  onEditSettings: () => void;
}>) {
  const { t } = useTranslation();
  const write = useCan("teaching.assignments.write");
  const intervene = useCan("teaching.attempts.intervene");
  const draft = status === "draft";
  if (!write && !(intervene && !draft)) return null;
  const unpublishable = (a.targetCount ?? 0) === 0;
  return (
    <>
      <HeaderMenu label={t("assignmentDetail.moreActions")} triggerRef={moreRef}>
        {write && draft && (
          <DropdownMenuItem asChild>
            <Link to={`/teacher/assignments/${a.id}/edit`}>
              <Pencil aria-hidden="true" />
              {t("assignments.detail.edit")}
            </Link>
          </DropdownMenuItem>
        )}
        {write && !draft && (
          <DropdownMenuItem onSelect={onEditSettings}>
            <Pencil aria-hidden="true" />
            {t("assignmentDetail.editSettings")}
          </DropdownMenuItem>
        )}
        {write && (
          <DropdownMenuItem onSelect={() => onDialog({ kind: "duplicate" })}>
            <Copy aria-hidden="true" />
            {t("assignments.list.duplicate")}
          </DropdownMenuItem>
        )}
        {intervene && !draft && (
          <DropdownMenuItem onSelect={() => onDialog({ kind: "reopenStudent" })}>
            <RotateCcw aria-hidden="true" />
            {t("assignmentDetail.reopenStudent.title")}
          </DropdownMenuItem>
        )}
        {write && status === "open" && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onSelect={() => {
                if (write && status === "open") onDialog({ kind: "close" });
              }}
            >
              <CircleStop aria-hidden="true" />
              {t("assignments.detail.closeEarly")}
            </DropdownMenuItem>
          </>
        )}
      </HeaderMenu>
      {write && !draft && (
        <Button
          ref={extendRef}
          variant="outline"
          onClick={() => onDialog({ kind: "extend" })}
        >
          <Clock aria-hidden="true" />
          {t("assignmentDetail.extend")}
        </Button>
      )}
      {write && draft && (
        <Button
          disabled={unpublishable}
          aria-disabled={pending || undefined}
          className="aria-disabled:opacity-50"
          onClick={() => {
            if (write && status === "draft" && !pending) onPublish();
          }}
        >
          <Send aria-hidden="true" />
          {t("assignments.detail.publish")}
        </Button>
      )}
    </>
  );
}

function HeaderMenu({
  label,
  triggerRef,
  children,
}: Readonly<{
  label: string;
  triggerRef: RefObject<HTMLButtonElement | null>;
  children: ReactNode;
}>) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button ref={triggerRef} variant="outline" size="icon-lg" aria-label={label}>
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-55">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function GradeAction({
  count,
  attemptId,
  onOpen,
}: Readonly<{
  count: number;
  attemptId: string | null | undefined;
  onOpen: (id: string) => void;
}>) {
  const { t } = useTranslation();
  const grade = useCan("teaching.grading");
  if (!grade || count === 0 || !attemptId) return null;
  return (
    <Button
      onClick={() => {
        if (grade && attemptId) onOpen(attemptId);
      }}
    >
      <SquarePen aria-hidden="true" />
      {t("assignmentDetail.gradeAnswers", { count })}
    </Button>
  );
}

function DetailProgress({
  monitor,
  stats,
  selected,
  onSelect,
}: Readonly<{
  monitor: UseQueryResult<Awaited<ReturnType<typeof getMonitor>>, Error>;
  stats: ReturnType<typeof assignmentStats> | null;
  selected: ProgressGroup | null;
  onSelect: (group: ProgressGroup | null) => void;
}>) {
  if (monitor.isError)
    return <ProgressCard state="error" onRetry={() => void monitor.refetch()} />;
  if (stats === null) return <ProgressCard state="pending" />;
  return (
    <ProgressCard
      state="ready"
      figures={stats}
      selected={selected}
      onSelect={onSelect}
    />
  );
}

function StudentsPanel({
  monitor,
  a,
  attemptId,
  onOpen,
  onRefresh,
}: Readonly<{
  monitor: UseQueryResult<Awaited<ReturnType<typeof getMonitor>>, Error>;
  a: Assignment;
  attemptId: string | null;
  onOpen: (id: string) => void;
  onRefresh: () => Promise<void>;
}>) {
  const { t } = useTranslation();
  if (monitor.isPending) return <ListSkeleton />;
  if (monitor.isError)
    return (
      <LoadError error={monitor.error} onRetry={() => void monitor.refetch()}>
        {t("monitor.loadFailed")}
      </LoadError>
    );
  return (
    <Monitor
      assignment={a}
      data={monitor.data}
      selectedAttempt={attemptId}
      onOpen={onOpen}
      onRefresh={onRefresh}
    />
  );
}
